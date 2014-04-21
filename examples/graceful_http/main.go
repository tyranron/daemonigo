// An example of graceful http server.
package main

import (
	daemon "github.com/tyranron/daemonigo"
	"log"
	"net/http"
	"os"
	"sync"
)

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

// Checks if received error during serving
// is appeared because of closing listener.
func isErrClosing(err error) bool {
	if opErr, ok := err.(*net.OpError); ok {
		err = opErr.Err
	}
	return "use of closed network connection" == err.Error()
}
