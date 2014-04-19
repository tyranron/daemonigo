package daemonigo

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
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
			Start()
		}
	},
	"stop": func() {
		switch isRunning, process, err := Status(); {
		case err != nil:
			printStatusErr(err)
		case !isRunning:
			fmt.Println(AppName + " is NOT running or already stopped")
		default:
			Stop(process)
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
			Stop(process)
		}
		Start()
	},
}

// Helper function to print errors of Status() function.
func printStatusErr(e error) {
	fmt.Println("Checking status of " + AppName + " failed")
	fmt.Println("Details:", e.Error())
}

// Helper function to operate with errors in actions.
func failed(e error) {
	fmt.Println("FAILED")
	fmt.Println("Details:", e.Error())
}

// Stops daemon process.
//
// This function can also be used when writing own daemon actions.
func Stop(process *os.Process) {
	fmt.Print("Stopping %s...", AppName)
	if err := process.Signal(os.Interrupt); err != nil {
		failed(err)
		return
	}
	for {
		time.Sleep(200 * time.Millisecond)
		switch isRunning, _, err := Status(); {
		case err != nil:
			printStatusErr(err)
			return
		case !isRunning:
			fmt.Println("OK")
			return
		}
	}
}

// Starts daemon process and waits 1 second.
// If daemonized process keeps running after this second
// then process seems to be successfully started.
//
// This function can also be used when writing own daemon actions.
func Start() {
	fmt.Printf("Starting %s...", AppName)
	path, err := filepath.Abs(AppPath)
	if err != nil {
		failed(err)
		return
	}
	cmd := exec.Command(path)
	cmd.Env = append(
		os.Environ(),
		fmt.Sprintf("%s=%s", EnvVarName, EnvVarValue),
	)
	if err = cmd.Start(); err != nil {
		failed(err)
		return
	}
	select {
	case <-func() chan bool {
		ch := make(chan bool)
		go func() {
			err = cmd.Wait()
			if err == nil {
				err = fmt.Errorf("%s stopped and not running", AppName)
			}
			failed(err)
			ch <- true
		}()
		return ch
	}():
	case <-time.After(time.Second):
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
