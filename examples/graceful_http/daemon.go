package main

import (
	daemon "github.com/tyranron/daemonigo"
)

func init() {
	printStatusErr := func(e error) {
		fmt.Println("Checking status of " + daemon.AppName + " failed")
		fmt.Println("Details:", e.Error())
	}
	printFailed := func(e error) {
		fmt.Println("FAILED")
		fmt.Println("Details:", e.Error())
	}

	daemon.SetAction("reload", func() {
		isRunning, process, err := daemon.Status()
		if err != nil {
			printStatusErr(err)
			return
		}
		if !isRunning {
			//todo:
		} else {
			fmt.Print("Reloading " + daemon.AppName + "...")
			if err := process.Signal(syscall.SIGHUP); err != nil {
				printFailed(err)
				return
			}
		}
	})
}
