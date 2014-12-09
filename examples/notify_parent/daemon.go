package main

import (
	"fmt"
	daemon "github.com/tyranron/daemonigo"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Overriding of default daemonic daemon actions.
func init() {
	daemon.AppName = "server"
	daemon.PidFile = "server.pid"

	printStatusErr := func(e error) {
		fmt.Printf("Checking status of %s FAILED\n", daemon.AppName)
		fmt.Printf("Details: %s\n", e.Error())
	}
	printFailed := func(e error) {
		fmt.Println("FAILED")
		fmt.Printf("Details: %s\n", e.Error())
	}

	// Helper function for custom daemon starting.
	start := func() {
		fmt.Printf("Starting %s...", daemon.AppName)
		sig := make(chan os.Signal)
		signal.Notify(sig, syscall.SIGUSR1)
		cmd, err := daemon.StartCommand()
		if err != nil {
			printFailed(err)
		}
		if err = cmd.Start(); err != nil {
			printFailed(err)
			return
		}
		select {
		case <-sig: // received "OK" signal from child process
			fmt.Println("OK")
		case <-time.After(10 * time.Second): // timeout for waiting signal
			fmt.Println("TIMEOUTED")
			fmt.Println("Error: signal from child process not received")
			fmt.Println("Check logs/application.log for details")
		case err := <-func() chan error {
			ch := make(chan error)
			go func() {
				err := cmd.Wait()
				if err == nil {
					err = fmt.Errorf("child process unexpectedly stopped")
				}
				ch <- err
			}()
			return ch
		}(): // child process unexpectedly stopped without sending signal
			printFailed(err)
		}
	}

	// Implements scenario of starting media-api.
	daemon.SetAction("start", func() {
		switch isRunning, _, err := daemon.Status(); {
		case err != nil:
			printStatusErr(err)
		case isRunning:
			fmt.Printf(
				"%s is already started and running now\n", daemon.AppName,
			)
		default:
			start()
		}
	})

	daemon.SetAction("restart", func() {
		isRunning, process, err := daemon.Status()
		if err != nil {
			printStatusErr(err)
			return
		}
		if isRunning {
			fmt.Printf("Stopping %s...", daemon.AppName)
			if err := daemon.Stop(process); err != nil {
				printFailed(err)
				return
			} else {
				fmt.Println("OK")
			}
		}
		start()
	})
}
