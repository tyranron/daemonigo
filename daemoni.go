// Package daemonigo provides a simple wrapper to daemonize applications.
package daemonigo

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Name of environment variable used to distinguish
// parent and daemonized processes.
var EnvVarName = "_DAEMONIGO"

// Value of environment variable used to distinguish
// parent and daemonized processes.
var EnvVarValue = "1"

// Path to daemon working directory.
// If not set, the current user directory will be used.
var WorkDir = ""

// Value of file mask for PID-file.
var PidFileMask os.FileMode = 0644

// Value of umask for daemonized process.
var Umask = 027

// Application name to daemonize.
// Used for printing in default daemon actions.
var AppName = "daemon"

// Path to application executable.
// Used only for default start/restart actions.
var AppPath = "./" + filepath.Base(os.Args[0])

// Absolute or relative path from working directory to PID file.
var PidFile = "daemon.pid"

// Pointer to PID file to keep file-lock alive.
var pidFile *os.File

// This function wraps application with daemonization.
// Returns isDaemon value to distinguish parent and daemonized processes.
func Daemonize() (isDaemon bool, err error) {
	const errLoc = "daemonigo.Daemonize()"
	isDaemon = os.Getenv(EnvVarName) == EnvVarValue
	if WorkDir != "" {
		if err = os.Chdir(WorkDir); err != nil {
			err = fmt.Errorf(
				"%s: changing working directory failed, reason -> %s",
				errLoc, err.Error(),
			)
			return
		}
	}
	if isDaemon {
		syscall.Umask(int(Umask))
		if _, err = syscall.Setsid(); err != nil {
			err = fmt.Errorf(
				"%s: setsid failed, reason -> %s", errLoc, err.Error(),
			)
			return
		}
		if pidFile, err = lockPidFile(); err != nil {
			err = fmt.Errorf(
				"%s: locking PID file failed, reason -> %s",
				errLoc, err.Error(),
			)
		}
	} else {
		flag.Usage = func() {
			arr := make([]string, 0, len(actions))
			for k, _ := range actions {
				arr = append(arr, k)
			}
			fmt.Fprintf(os.Stderr, "Usage: %s {%s}\n",
				os.Args[0], strings.Join(arr, "|"),
			)
			flag.PrintDefaults()
		}
		if !flag.Parsed() {
			flag.Parse()
		}
		action, exist := actions[flag.Arg(0)]
		if exist {
			action()
		} else {
			flag.Usage()
		}
	}
	return
}

// Locks PID file with a file lock.
// Keeps PID file open until applications exits.
func lockPidFile() (pidFile *os.File, err error) {
	var file *os.File
	file, err = os.OpenFile(
		PidFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, PidFileMask,
	)
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

// Unlocks PID file (locked by current daemonized process) and closes this file.
//
// This function can be useful for graceful restarts or other
// untrivial scenarios, but usually there is no need to use it.
func UnlockPidFile() {
	if pidFile != nil {
		syscall.Flock(int(pidFile.Fd()), syscall.LOCK_UN)
		pidFile.Close()
	}
}

// Checks status of daemonized process.
// Can be used in daemon actions to perate with daemonized process.
func Status() (isRunning bool, pr *os.Process, e error) {
	const errLoc = "daemonigo.Status()"
	var (
		err  error
		file *os.File
	)

	file, err = os.Open(PidFile)
	if err != nil {
		if !os.IsNotExist(err) {
			e = fmt.Errorf(
				"%s: could not open PID file, reason -> %s",
				errLoc, err.Error(),
			)
		}
		return
	}
	defer file.Close()
	fd := int(file.Fd())
	if err = syscall.Flock(
		fd, syscall.LOCK_EX|syscall.LOCK_NB,
	); err != syscall.EWOULDBLOCK {
		if err == nil {
			syscall.Flock(fd, syscall.LOCK_UN)
		} else {
			e = fmt.Errorf(
				"%s: PID file locking attempt failed, reason -> %s",
				errLoc, err.Error(),
			)
		}
		return
	}

	isRunning = true
	var n, pid int
	content := make([]byte, 128)
	n, err = file.Read(content)
	if err != nil && err != io.EOF {
		e = fmt.Errorf(
			"%s: could not read from PID file, reason -> %s",
			errLoc, err.Error(),
		)
		return
	}
	pid, err = strconv.Atoi(string(content[:n]))
	if n < 1 || err != nil {
		e = fmt.Errorf(
			"%s: bad PID format, PID file is possibly corrupted", errLoc,
		)
		return
	}
	pr, err = os.FindProcess(pid)
	if err != nil {
		e = fmt.Errorf(
			"%s: cannot find process by PID, reason -> %s", errLoc, err.Error(),
		)
	}

	return
}

// Prepares and returns command for starting daemonized process.
//
// This function can also be used when writing your own daemon actions.
func StartCommand() (*exec.Cmd, error) {
	const errLoc = "daemonigo.StartCommand()"
	path, err := filepath.Abs(AppPath)
	if err != nil {
		return nil, fmt.Errorf(
			"%s: failed to resolve absolute path of %s, reason -> %s",
			errLoc, AppName, err.Error(),
		)
	}
	cmd := exec.Command(path)
	cmd.Env = append(
		os.Environ(), fmt.Sprintf("%s=%s", EnvVarName, EnvVarValue),
	)
	return cmd, nil
}

// Starts daemon process and waits timeout number of seconds.
// If daemonized process keeps running after timeout seconds passed
// then process seems to be successfully started.
//
// This function can also be used when writing your own daemon actions.
func Start(timeout uint8) (e error) {
	const errLoc = "daemonigo.Start()"
	cmd, err := StartCommand()
	if err != nil {
		return fmt.Errorf(
			"%s: failed to create daemon start command, reason -> %s",
			errLoc, err.Error(),
		)
	}
	if err = cmd.Start(); err != nil {
		return fmt.Errorf(
			"%s: failed to start %s, reason -> %s",
			errLoc, AppName, err.Error(),
		)
	}
	select {
	case <-func() chan bool {
		ch := make(chan bool)
		go func() {
			if err := cmd.Wait(); err != nil {
				e = fmt.Errorf(
					"%s: %s running failed, reason -> %s",
					errLoc, AppName, err.Error(),
				)
			} else {
				e = fmt.Errorf(
					"%s: %s stopped and not running", errLoc, AppName,
				)
			}
			ch <- true
		}()
		return ch
	}():
	case <-time.After(time.Duration(timeout) * time.Second):
	}
	return
}

// Stops daemon process.
// Sends signal os.Interrupt to daemonized process.
//
// This function can also be used when writing your own daemon actions.
func Stop(process *os.Process) (e error) {
	const errLoc = "daemonigo.Stop()"
	if err := process.Signal(os.Interrupt); err != nil {
		e = fmt.Errorf(
			"%s: failed to send interrupt signal to %s, reason -> %s",
			errLoc, AppName, err.Error(),
		)
		return
	}
	for {
		time.Sleep(200 * time.Millisecond)
		switch isRunning, _, err := Status(); {
		case err != nil:
			e = fmt.Errorf(
				"%s: checking status of %s failed, reason -> %s",
				errLoc, AppName, err.Error(),
			)
			return
		case !isRunning:
			return
		}
	}
}
