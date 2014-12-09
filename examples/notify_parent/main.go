// An example of simple "Hello, World!" HTTP server with bulletproof starting.
//
//
// Overview
//
// Simple  HTTP server that runs on 8889 port and says "Hello, World!".
// Overrides "start" action of daemonigo package to ensure that server
// really started successfully.
//
//
// Idea
//
// By default, daemonigo "start" action waits 1 second and if during this
// period daemonized process doesn't stop, daemonigo will assume that
// daemonized process started successfully. However, sometimes application
// can have "heavy" initialization phase with connecting to different remote
// server or other complicated tasks, so waiting 1 second is not enough
// to be ensured that application started successfully and everything is fine.
// The trick is simple: after everything was initialized successfully in
// daemonized process we send some signal to parent process that
// everything is OK.
//
//
// Usage
//
// To start server:
//		./server start
// To stop server:
//		./server stop
// To restart server:
//		./server restart
package main

import (
	daemon "github.com/tyranron/daemonigo"
	"log"
	"net"
	"net/http"
	"os"
	"syscall"
	"time"
)

func main() {
	// Daemonizing http server.
	switch isDaemon, err := daemon.Daemonize(); {
	case !isDaemon:
		return
	case err != nil:
		log.Fatalf("main(): could not start daemon, reason -> %s", err.Error())
	}
	// From now we are running in daemon process.

	// Imitating "heavy" initialization phase.
	time.Sleep(time.Second)

	// Creating "Hello, World!" HTTP server.
	http.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("Hello, World!"))
	})
	listener, err := net.Listen("tcp", ":8889")
	if err != nil {
		log.Fatalf(
			"main(): failed to listen on port :8889, reason -> %s", err.Error(),
		)
	}
	defer listener.Close()
	stoppedRunning := make(chan struct{})
	go func() {
		defer close(stoppedRunning)
		if err := http.Serve(listener, nil); err != nil && !isErrClosing(err) {
			log.Fatalf(
				"main(): failed to serve listener, reason -> %s", err.Error(),
			)
		}
	}()

	// Imitating more "heavy" initialization tasks.
	time.Sleep(time.Second)

	// Notifying parent process that we have started successfully.
	if err := syscall.Kill(os.Getppid(), syscall.SIGUSR1); err != nil {
		log.Fatalf(
			"main(): notifying parent proccess failed, reason -> %s",
			err.Error(),
		)
	}

	// Preventing main goroutine from closing while HTTP server is running.
	<-stoppedRunning
}

// Checks if received error during serving
// is appeared because of closing listener.
func isErrClosing(err error) bool {
	if opErr, ok := err.(*net.OpError); ok {
		err = opErr.Err
	}
	return "use of closed network connection" == err.Error()
}
