package main

import (
	daemon "github.com/tyranron/daemonigo"
)

func init() {
	daemon.SetAction("reload", func() {

	})
}
