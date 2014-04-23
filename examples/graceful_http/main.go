// An example of graceful http server with zero-downtime reload.
package main

import (
	"fmt"
	daemon "github.com/tyranron/daemonigo"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
)

const envVarName = "_GO_FD"

func main() {
	// Daemonizing http server.
	switch isDaemon, err := daemon.Daemonize(); {
	case !isDaemon:
		return
	case err != nil:
		log.Fatalf("main(): could not start daemon, reason -> %s", err.Error())
		return
	}
	// From now we are running in daemon process.

	// Starting listen tcp on 8080 port.
	listener, hasPrev, err := previousListener()
	if err != nil {
		if hasPrev {
			log.Fatalf("main(): failed to resume listener, reason -> %s", err.Error())
		}
		if listener, err = net.Listen("tcp", ":8080"); err != nil {
			log.Fatalf("main(): failed to listen, reason -> %s", err.Error())
		}
	}

	// Listen OS signals in separate goroutine.
	go listenSignals(listener)

	// Creating a simple one-page http server.
	PID := syscall.Getpid()
	waiter := new(sync.WaitGroup)
	s := &http.Server{Addr: ":8080"}
	http.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		waiter.Add(1)
		defer waiter.Done()
		w.Write([]byte(fmt.Sprintf("Hi! I am graceful http server! My PID is %d", PID)))
	})
	if err := s.Serve(listener); err != nil && !isErrClosing(err) {
		log.Fatalf("main(): failed to serve listener, reason -> %s", err.Error())
	}

	// Waiting all requests to be finished.
	waiter.Wait()
}

// Helper function which checks environment variables
// for socket descriptors and starts listening on it if any.
func previousListener() (l net.Listener, hasPrev bool, e error) {
	const errLoc = "main.previousListener()"
	var fd uintptr
	if _, e = fmt.Sscan(os.Getenv(envVarName), &fd); e != nil {
		e = fmt.Errorf("%s: could not read file descriptor from environment, reason -> %s", errLoc, e.Error())
		return
	}
	hasPrev = true
	if l, e = net.FileListener(os.NewFile(fd, "parent socket")); e != nil {
		e = fmt.Errorf("%s: could not listen on old file descriptor, reason -> %s", errLoc, e.Error())
		return
	}
	switch l.(type) {
	case *net.TCPListener, *net.UnixListener:
	default:
		e = fmt.Errorf("%s: file descriptor is %T not *net.TCPListener or *net.UnixListener", errLoc, l)
		return
	}
	if e = syscall.Close(int(fd)); e != nil {
		e = fmt.Errorf("%s: failed to close old file descriptor, reason -> %s", errLoc, e.Error())
	}
	return
}

// Checks if received error during serving
// is appeared because of closing listener.
func isErrClosing(err error) bool {
	if opErr, ok := err.(*net.OpError); ok {
		err = opErr.Err
	}
	return "use of closed network connection" == err.Error()
}

// A goroutine which listens OS signals.
func listenSignals(l net.Listener) {
	sigChan := make(chan os.Signal)
	signal.Notify(sigChan,
		syscall.SIGHUP, syscall.SIGINT,
		syscall.SIGQUIT, syscall.SIGTERM,
		syscall.SIGUSR1, syscall.SIGUSR2,
	)
	for {
		switch sig := <-sigChan; sig {
		case syscall.SIGHUP:
			if err := reload(l); err != nil {
				log.Println(err.Error())
			}
		default:
			l.Close()
		}
	}
}

func reload(l net.Listener) error {
	const errLoc = "main.reload()"

	// Making duplicate for socket descriptor
	// to use them in child process.
	file, err := (l.(*net.TCPListener)).File()
	if err != nil {
		return fmt.Errorf("%s: failed to get file of listener, reason -> %s", errLoc, err.Error())

	}
	fd, err := syscall.Dup(int(file.Fd()))
	if err != nil {
		return fmt.Errorf("%s: failed to dup(2) listener, reason -> %s", errLoc, err.Error())
	}
	if err := os.Setenv(envVarName, fmt.Sprint(fd)); err != nil {
		return fmt.Errorf("%s: failed to write fd into environment variable, reason -> %s", errLoc, err.Error())
	}

	// Unlock PID file to start normally child process.
	daemon.UnlockPidFile()

	// Start child process.
	cmd := exec.Command(daemon.AppPath)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%s: failed to start child process, reason -> %s", errLoc, err.Error())
	}

	// Close current listener.
	l.Close()

	return nil
}
