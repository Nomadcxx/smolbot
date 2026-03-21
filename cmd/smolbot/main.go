package main

import (
	"fmt"
	"os"
)

var version = "dev"

func main() {
	cmd := NewRootCmd(version)
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
