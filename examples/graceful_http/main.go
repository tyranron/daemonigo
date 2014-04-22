// An example of graceful http server.
package main

import (
	daemon "github.com/tyranron/daemonigo"
	"log"
	"net/http"
	"os"
	"sync"
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

	// Creating a simple one-page http server.
	waiter := new(sync.WaitGroup)
	s := &http.Server{Addr: ":8080"}
	http.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		waiter.Add(1)
		defer waiter.Done()
		w.Write("Hi! I am graceful http server!")
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
	const errLoc = "main.Listener()"
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
		e = fmt.Errorf("%s: failed to close file descriptor, reason -> %s", errLoc, e.Error())
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
	switch sig := <-sigChan; sig {
	case syscall.SIGHUP:
		reload(l)
	default:
		l.Close()
	}
}


func reload(l net.Listener) {
	l.Close()
}
