// A simple echo server implementation from net.Listener official documentation.
//
//
// Overview
//
// Simple echo server that runs on 2000 port.
// Uses all default actions of daemonigo package.
//
//
// Build
//
// Simply with go build tool:
//		go build -o .echoserv examples/echo_server/*
//
//
// Usage
//
// To start server:
//		./echoserv start
// To stop server:
//		./echoserv stop
// To restart server:
//		./echoserv restart
package main

import (
	daemon "github.com/tyranron/daemonigo"
	"io"
	"log"
	"net"
)

func main() {
	// Daemonizing echo server application.
	switch isDaemon, err := daemon.Daemonize(); {
	case !isDaemon:
		return
	case err != nil:
		log.Fatalf("main(): could not start daemon, reason -> %s", err.Error())
	}
	// From now we are running in daemon process.

	// Listen on TCP port 2000 on all interfaces.
	l, err := net.Listen("tcp", ":2000")
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()
	for {
		// Wait for a connection.
		conn, err := l.Accept()
		if err != nil {
			log.Fatal(err)
		}
		// Handle the connection in a new goroutine.
		// The loop then returns to accepting, so that
		// multiple connections may be served concurrently.
		go func(c net.Conn) {
			// Echo all incoming data.
			io.Copy(c, c)
			// Shut down the connection.
			c.Close()
		}(conn)
	}
}
