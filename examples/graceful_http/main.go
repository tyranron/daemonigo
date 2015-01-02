// An example of graceful HTTP server with zero-downtime reload.
//
//
// Overview
//
// Simple graceful HTTP server that runs on 8888 port.
//
//
// Idea
//
// The idea of zero-downtime reload is to reuse socket descriptor in
// new process: parent daemon process duplicates (by dup(2)) socket of
// its listener, spawns new child daemon process and gives descriptor
// of socket to child with environment variable.
// After child daemon process succeeds to listen on parent socket, it sends
// signal to parent daemon process to stop to listen.
//
// Testing
//
// To test server you can use "test" daemon option. Just run command:
//		./graceful test
// Now you can play into another terminal with commands
//		./graceful reload
//		./graceful restart
// and you will see output like:
//		...................
//		...............E...
// Where "." means successful request and "E" means failed request.
//
// By default test application makes request every 10 ms.
// You can change frequency of requests by "-ms" flag, like:
//		./grace -ms=1 test
package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"

	daemon "github.com/tyranron/daemonigo"
)

// Name of environment variable, which holds
// file descriptor of listener socket connection for child process.
const envVarName = "_GO_FD"

func main() {
	// Daemonizing http server.
	switch isDaemon, err := daemon.Daemonize(); {
	case !isDaemon:
		return
	case err != nil:
		log.Fatalf("main(): could not start daemon, reason -> %s", err.Error())
	}
	// From now we are running in daemon process.

	// Starting listen tcp on 8888 port.
	listener, hasPrev, err := previousListener()
	if err != nil {
		if hasPrev {
			log.Fatalf(
				"main(): failed to resume listener, reason -> %s", err.Error(),
			)
		}
		if listener, err = net.Listen("tcp", ":8888"); err != nil {
			log.Fatalf("main(): failed to listen, reason -> %s", err.Error())
		}
	}
	httpServer := &http.Server{}

	// Listen OS signals in separate goroutine.
	go listenSignals(listener, httpServer)

	// Creating a simple one-page http server.
	PID := os.Getppid()
	waiter := new(sync.WaitGroup)
	http.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		waiter.Add(1)
		fmt.Fprintf(w, "Hi! I am graceful http server! My PID is %d", PID)
		waiter.Done()
	})
	waiter.Add(1)
	go func() {
		if err := httpServer.Serve(listener); err != nil && !isErrClosing(err) {
			log.Fatalf(
				"main(): failed to serve listener, reason -> %s", err.Error(),
			)
		}
		waiter.Done()
	}()

	// If was started by "reload" option.
	if hasPrev {
		if err := syscall.Kill(PID, syscall.SIGUSR1); err != nil {
			log.Printf(
				"main(): failed to notify parent daemon procces, reason -> %s",
				err.Error(),
			)
		}
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
		e = fmt.Errorf(
			"%s: could not read file descriptor from environment, reason -> %s",
			errLoc, e.Error(),
		)
		return
	}
	hasPrev = true
	if l, e = net.FileListener(os.NewFile(fd, "parent socket")); e != nil {
		e = fmt.Errorf(
			"%s: could not listen on old file descriptor, reason -> %s",
			errLoc, e.Error(),
		)
		return
	}
	switch l.(type) {
	case *net.TCPListener:
	default:
		e = fmt.Errorf(
			"%s: file descriptor is %T not *net.TCPListener", errLoc, l,
		)
		return
	}
	if e = syscall.Close(int(fd)); e != nil {
		e = fmt.Errorf(
			"%s: failed to close old file descriptor, reason -> %s",
			errLoc, e.Error(),
		)
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
func listenSignals(l net.Listener, srv *http.Server) {
	sigChan := make(chan os.Signal)
	signal.Notify(sigChan,
		syscall.SIGHUP, syscall.SIGINT,
		syscall.SIGQUIT, syscall.SIGTERM,
		syscall.SIGUSR1, syscall.SIGUSR2,
	)
	for {
		switch sig := <-sigChan; sig {
		case syscall.SIGHUP:
			if err := reload(l, srv); err != nil {
				log.Println(err.Error())
			}
		default:
			l.Close()
		}
	}
}

// Implements zero-downtime application reload.
func reload(l net.Listener, httpServer *http.Server) error {
	const errLoc = "main.reload()"

	// Making duplicate for socket descriptor
	// to use them in child process.
	file, err := (l.(*net.TCPListener)).File()
	if err != nil {
		return fmt.Errorf(
			"%s: failed to get file of listener, reason -> %s",
			errLoc, err.Error(),
		)
	}
	fd, err := syscall.Dup(int(file.Fd()))
	file.Close()
	if err != nil {
		return fmt.Errorf(
			"%s: failed to dup(2) listener, reason -> %s", errLoc, err.Error(),
		)
	}
	if err := os.Setenv(envVarName, fmt.Sprint(fd)); err != nil {
		return fmt.Errorf(
			"%s: failed to write fd into environment variable, reason -> %s",
			errLoc, err.Error(),
		)
	}

	// Unlock PID file to start normally child process.
	daemon.UnlockPidFile()

	// Start child process.
	cmd := exec.Command(daemon.AppPath)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf(
			"%s: failed to start child process, reason -> %s",
			errLoc, err.Error(),
		)
	}

	select {
	// Waiting for notification from child process that it starts successfully.
	// In real application it's better to move generation of chan for this case
	// before calling cmd.Start() to be sure to catch signal in any case.
	case <-func() <-chan os.Signal {
		sig := make(chan os.Signal)
		signal.Notify(sig, syscall.SIGUSR1)
		return sig
	}():
	// If child process stopped without sending signal.
	case <-func() chan struct{} {
		ch := make(chan struct{}, 1)
		go func() {
			cmd.Wait()
			ch <- struct{}{}
		}()
		return ch
	}():
		err = fmt.Errorf("%s: child process stopped unexpectably", errLoc)
	// Timeout for waiting signal from child process.
	case <-time.After(10 * time.Second):
		err = fmt.Errorf(
			"%s: child process is not responding, closing by timeout", errLoc,
		)
	}

	// Dealing with keep-alive connections.
	httpServer.SetKeepAlivesEnabled(false)
	time.Sleep(100 * time.Millisecond)

	// Close current listener (stop server in fact).
	l.Close()
	return err
}
