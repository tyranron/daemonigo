package daemonigo

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// Name of environment variable used to distinguish
// parent and daemonized processes.
var EnvVarName = "_DAEMONIGO"
// Value of environment variable used to distinguish
// parent and daemonized processes.
var EnvVarValue = "1"

// Value of file mask for PID-file.
var PidFileMask = 0644
// Value of umask for daemonized process.
var Umask = 027


var (
	Name    = "daemon"
	PidFile = "daemon.pid"
	AppPath = "./" + filepath.Base(os.Args[0])

	pidFile *os.File
)

func Daemonize(workDir string) (isDeamon bool, err error) {
	const errLoc = "daemonic.Daemonize()"
	isDeamon = os.Getenv(EnvVarName) == EnvVarValue
	if len(workDir) != 0 {
		if err = os.Chdir(workDir); err != nil {
			err = fmt.Errorf("%s: changing working directory failed, reason -> %s", err.Error())
			return
		}
	}
	if isDeamon {
		syscall.Umask(int(Umask))
		if _, err = syscall.Setsid(); err != nil {
			err = fmt.Errorf("%s: setsid failed, reason -> %s", err.Error())
			return
		}
		if pidFile, err = lockPidFile(); err != nil {
			err = fmt.Errorf("%s: locking PID file failed, reason -> %s: ", err.Error())
		}
	} else {
		if !flag.Parsed() {
			flag.Parse()
		}
		action, exist := actions[flag.Arg(0)]
		if exist {
			action()
		} else {
			arr := make([]string, 0, len(actions))
			for k, _ := range actions {
				arr = append(arr, k)
			}
			fmt.Println("Usage: " + os.Args[0] + " {" + strings.Join(arr, "|") + "}")
		}
	}
	return
}

func lockPidFile() (pidFile *os.File, err error) {
	var file *os.File
	file, err = os.OpenFile(PidFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, PidFileMask)
	if err != nil {
		return
	}
	defer func() {
		// file must be open whole runtime to keep lock on itself
		if err != nil {
			file.Close()
		}
	}()

	if err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		return
	}
	var fileLen int
	fileLen, err = fmt.Fprint(file, os.Getpid())
	if err != nil {
		return
	}
	if err = file.Truncate(int64(fileLen)); err != nil {
		return
	}

	return file, err
}

func UnlockPidFile() {
	if pidFile != nil {
		syscall.Flock(int(pidFile.Fd()), syscall.LOCK_UN)
		pidFile.Close()
	}
}

func Status() (isRunning bool, pr *os.Process, e error) {
	var (
		err  error
		file *os.File
	)

	file, err = os.Open(PidFile)
	if err != nil {
		if !os.IsNotExist(err) {
			e = fmt.Errorf("could not open PID file: " + err.Error())
		}
		return
	}
	defer file.Close()
	fd := int(file.Fd())
	if err = syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB); err != syscall.EWOULDBLOCK {
		if err == nil {
			syscall.Flock(fd, syscall.LOCK_UN)
		} else {
			e = fmt.Errorf("PID file locking attempt failed: " + err.Error())
		}
		return
	}

	isRunning = true
	var n, pid int
	content := make([]byte, 128)
	n, err = file.Read(content)
	if err != nil && err != io.EOF {
		e = fmt.Errorf("could not read from PID file: " + err.Error())
		return
	}
	pid, err = strconv.Atoi(string(content[:n]))
	if n < 1 || err != nil {
		e = fmt.Errorf("bad PID format, PID file is possibly corrupted")
		return
	}
	pr, err = os.FindProcess(pid)
	if err != nil {
		e = fmt.Errorf("cannot find process by PID: " + err.Error())
	}

	return
}
