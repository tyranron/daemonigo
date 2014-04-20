package daemonigo

import (
	"fmt"
	"os"
)

// Daemon default actions.
// Can be changed with SetAction() and RemoveAction() functions.
var actions = map[string]func(){
	"start": func() {
		switch isRunning, _, err := Status(); {
		case err != nil:
			printStatusErr(err)
		case isRunning:
			fmt.Println(AppName + " is already started and running now")
		default:
			start()
		}
	},
	"stop": func() {
		switch isRunning, process, err := Status(); {
		case err != nil:
			printStatusErr(err)
		case !isRunning:
			fmt.Println(AppName + " is NOT running or already stopped")
		default:
			stop(process)
		}
	},
	"status": func() {
		switch isRunning, process, err := Status(); {
		case err != nil:
			printStatusErr(err)
		case !isRunning:
			fmt.Println(AppName + " is NOT running")
		default:
			fmt.Printf("%s is running with PID %d\n", AppName, process.Pid)
		}
	},
	"restart": func() {
		isRunning, process, err := Status()
		if err != nil {
			printStatusErr(err)
			return
		}
		if isRunning {
			stop(process)
		}
		start()
	},
}

// Helper function to print errors of Status() function.
func printStatusErr(e error) {
	fmt.Println("Checking status of " + AppName + " failed")
	fmt.Println("Details:", e.Error())
}

// Helper function to operate with errors printing in actions.
func failed(e error) {
	fmt.Println("FAILED")
	fmt.Println("Details:", e.Error())
}

// Helper function which wraps Stop() with printing
// for using in daemon default actions.
func stop(process *os.Process) {
	fmt.Printf("Stopping %s...", AppName)
	if err := Stop(process); err != nil {
		failed(err)
	} else {
		fmt.Println("OK")
	}
}

// Helper function which wraps Start() with printing
// for using in daemon default actions.
func start() {
	fmt.Printf("Starting %s...", AppName)
	if err := Start(1); err != nil {
		failed(err)
	} else {
		fmt.Println("OK")
	}
}

// Sets new daemon action with given name or overrides previous.
//
// This function is not concurrent safe, so you must synchronize
// its calls in case of usage in multiple goroutines.
func SetAction(name string, action func()) {
	if name == "" {
		panic("daemonigo.SetAction(): name cannot be empty")
	}
	if action == nil {
		panic("daemonigo.SetAction(): action cannot be nil")
	}
	actions[name] = action
}

// Removes daemon action with given name.
//
// This function is not concurrent safe, so you must synchronize
// its calls in case of usage in multiple goroutines.
func RemoveAction(name string) {
	delete(actions, name)
}
