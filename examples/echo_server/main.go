package main

import (
	daemon "github.com/tyranron/daemonigo"
	"io"
	"log"
	"net"
)

func main() {
	switch isDaemon, err := daemon.Daemonize(); {
	case !isDaemon:
		return
	case err != nil:
		log.Fatalf("Error: could not start daemon, reason -> %s", err.Error())
		return
	}

	l, err := net.Listen("tcp", ":2000")
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go func(c net.Conn) {
			io.Copy(c, c)
			c.Close()
		}(conn)
	}
}
