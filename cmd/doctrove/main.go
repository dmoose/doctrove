package main

import "github.com/dmoose/doctrove/cli"

// version is set at build time via -ldflags.
var version = "dev"

func main() {
	cli.SetVersion(version)
	cli.Execute()
}
