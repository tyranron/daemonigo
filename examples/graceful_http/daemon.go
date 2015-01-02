package main

import (
	"flag"
	"fmt"
	"net/http"
	"syscall"
	"time"

	daemon "github.com/tyranron/daemonigo"
)

func init() {
	// Setting up daemon properties.
	daemon.AppName = "Graceful HTTP Server"
	daemon.PidFile = "graceful.pid"

	// Helper functions for printing.
	printStatusErr := func(e error) {
		fmt.Println("Checking status of " + daemon.AppName + " failed")
		fmt.Println("Details:", e.Error())
	}
	printFailed := func(e error) {
		fmt.Println("FAILED")
		fmt.Println("Details:", e.Error())
	}

	// Implementation of reload action.
	daemon.SetAction("reload", func() {
		isRunning, process, err := daemon.Status()
		if err != nil {
			printStatusErr(err)
			return
		}
		if !isRunning {
			fmt.Println(daemon.AppName + " is NOT running now")
			fmt.Printf("Starting %s...", daemon.AppName)
			if err := daemon.Start(1); err != nil {
				printFailed(err)
			} else {
				fmt.Println("OK")
			}
		} else {
			fmt.Printf("Reloading %s...", daemon.AppName)
			if err := process.Signal(syscall.SIGHUP); err != nil {
				printFailed(err)
				return
			}
			select {
			case <-func(prevPid int) chan bool {
				ch := make(chan bool)
				go func() {
					defer close(ch)
					for {
						time.Sleep(200 * time.Millisecond)
						switch isRunning, process, err := daemon.Status(); {
						case err != nil:
							printStatusErr(err)
							return
						case isRunning && (process.Pid != prevPid):
							fmt.Println("OK")
							return
						}
					}
				}()
				return ch
			}(process.Pid):
			case <-time.After(10 * time.Second):
				printStatusErr(fmt.Errorf(
					"checking new process timed out, see application logs",
				))
			}
		}
	})

	// A simple program to test server during reloads.
	var ms uint
	flag.UintVar(
		&ms, "ms", 10,
		"frequency of requests in milliseconds, must be positive integer",
	)
	daemon.SetAction("test", func() {
		if ms == 0 {
			ms = 10
		}
		errs, oks := 0, 0
		for i := 0; i < 10000; i++ {
			if r, err := http.Get("http://127.0.0.1:8888"); err != nil {
				print("E")
				errs++
			} else {
				print(".")
				oks++
				r.Body.Close()
			}
			time.Sleep(time.Duration(ms) * time.Millisecond)
		}
		println("\n---------------------------")
		println("Succeed:", oks, "Errors:", errs)
	})
}
