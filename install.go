// Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc>
// See LICENSE for licensing information

package main

import (
	"fmt"
	"os"

	"mvdan.cc/fdroidcl/adb"
	"mvdan.cc/fdroidcl/fdroid"
)

var cmdInstall = &Command{
	UsageLine: "install <appid...>",
	Short:     "Install or upgrade an app",
}

var (
	installUpdates = cmdInstall.Fset.Bool("u", false, "Upgrade all installed apps")
	installDryRun  = cmdInstall.Fset.Bool("n", false, "Only print the operations that would be done")
)

func init() {
	cmdInstall.Run = runInstall
}

func runInstall(args []string) error {
	if *installUpdates {
		if len(args) > 0 {
			return fmt.Errorf("-u can only be used without arguments")
		}
	} else if len(args) < 1 {
		return fmt.Errorf("no package names given")
	}
	device, err := oneDevice()
	if err != nil {
		return err
	}
	inst, err := device.Installed()
	if err != nil {
		return err
	}

	if *installUpdates {
		apps, err := loadIndexes()
		if err != nil {
			return err
		}
		apps = filterAppsUpdates(apps, inst, device)
		if len(apps) == 0 {
			fmt.Fprintln(os.Stderr, "All apps up to date.")
		}
		return downloadAndDo(apps, device)
	}

	apps, err := findApps(args)
	if err != nil {
		return err
	}
	var toInstall []fdroid.App
	for _, app := range apps {
		p, e := inst[app.PackageName]
		if !e {
			// installing an app from scratch
			toInstall = append(toInstall, app)
			continue
		}
		suggested := app.SuggestedApk(device)
		if suggested == nil {
			return fmt.Errorf("no suitable APKs found for %s", app.PackageName)
		}
		if p.VersCode >= suggested.VersCode {
			fmt.Printf("%s is up to date\n", app.PackageName)
			// app is already up to date
			continue
		}
		// upgrading an existing app
		toInstall = append(toInstall, app)
	}
	return downloadAndDo(toInstall, device)
}

func downloadAndDo(apps []fdroid.App, device *adb.Device) error {
	type downloaded struct {
		apk  *fdroid.Apk
		path string
	}
	toInstall := make([]downloaded, len(apps))
	for i, app := range apps {
		apk := app.SuggestedApk(device)
		if apk == nil {
			return fmt.Errorf("no suitable APKs found for %s", app.PackageName)
		}
		if *installDryRun {
			fmt.Printf("install %s:%d\n", app.PackageName, apk.VersCode)
			continue
		}
		path, err := downloadApk(apk)
		if err != nil {
			return err
		}
		toInstall[i] = downloaded{apk: apk, path: path}
	}
	if *installDryRun {
		return nil
	}
	for _, t := range toInstall {
		if err := installApk(device, t.apk, t.path); err != nil {
			return err
		}
	}
	return nil
}

func installApk(device *adb.Device, apk *fdroid.Apk, path string) error {
	fmt.Printf("Installing %s\n", apk.AppID)
	if err := device.Install(path); err != nil {
		return fmt.Errorf("could not install %s: %v", apk.AppID, err)
	}
	return nil
}
