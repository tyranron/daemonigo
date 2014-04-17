package daemonigo

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

var actMux *sync.Mutex = &sync.Mutex{}
var actions map[string]func() = map[string]func(){
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
			fmt.Printf(AppName+" is running with PID %d\n", process.Pid)
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

func printStatusErr(e error) {
	fmt.Println("Checking status of " + AppName + " failed")
	fmt.Println("Details:", e.Error())
}

func Stop(process *os.Process) {
	fmt.Print("Stopping " + AppName + "...")
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

func Start() {
	fmt.Print("Starting " + AppName + "...")
	path, err := filepath.Abs(AppPath)
	if err != nil {
		failed(err)
		return
	}
	cmd := PrepareCommand(path)
	if err = cmd.Start(); err != nil {
		failed(err)
		return
	}
	ch := make(chan bool)
	go func() {
		time.Sleep(1 * time.Second)
		fmt.Println("OK")
		ch <- true
	}()
	go func() {
		err = cmd.Wait()
		if err == nil {
			err = fmt.Errorf("daemon stopped and not running")
		}
		failed(err)
		ch <- true
	}()
	<-ch
}

func failed(e error) {
	fmt.Println("FAILED")
	fmt.Println("Details:", e.Error())
}

func PrepareCommand(path string) (cmd *exec.Cmd) {
	cmd = exec.Command(path)
	cmd.Env = append(
		os.Environ(),
		fmt.Sprintf("%s=%s", EnvVarName, EnvVarValue),
	)
	return
}

func HandleFunc(pattern string, handler func()) {
	actMux.Lock()
	defer actMux.Unlock()
	if pattern == "" {
		panic("daemonic: pattern cannot be empty")
	}
	if handler == nil {
		panic("daemonic: nil handler")
	}
	actions[pattern] = handler
}
