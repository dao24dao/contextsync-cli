package main

import (
	"os"

	"contextsync/internal/cli"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if err := cli.Execute(version, commit, date); err != nil {
		os.Exit(1)
	}
}
