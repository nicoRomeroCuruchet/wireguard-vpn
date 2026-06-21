package main

import (
	"fmt"
	"io"
	"os"
)

const version = "hsl 0.1.0-dev"

func main() {
	os.Exit(dispatch(os.Args[1:], os.Stdout, os.Stderr))
}

func dispatch(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: hsl <server|client|version> [flags]")
		return 2
	}
	switch args[0] {
	case "version":
		fmt.Fprintln(stdout, version)
		return 0
	case "server":
		fmt.Fprintln(stderr, "server: not implemented yet")
		return 1
	case "client":
		fmt.Fprintln(stderr, "client: not implemented yet")
		return 1
	default:
		fmt.Fprintf(stderr, "unknown subcommand %q\n", args[0])
		return 2
	}
}
