package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/nromero/hsl/internal/server"
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
		return runServer(args[1:], stderr)
	case "client":
		fmt.Fprintln(stderr, "client: not implemented yet")
		return 1
	default:
		fmt.Fprintf(stderr, "unknown subcommand %q\n", args[0])
		return 2
	}
}

func runServer(args []string, stderr io.Writer) int {
	fs := flag.NewFlagSet("server", flag.ContinueOnError)
	fs.SetOutput(stderr)
	addr := fs.String("addr", ":8080", "HTTP listen address")
	db := fs.String("db", "/var/lib/hsl/hsl.db", "SQLite database path")
	endpoint := fs.String("endpoint", "", "public WireGuard endpoint host:port (required)")
	keyPath := fs.String("key", "/var/lib/hsl/identity.key", "hub WireGuard private key path")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *endpoint == "" {
		fmt.Fprintln(stderr, "server: --endpoint is required (e.g. 1.2.3.4:51820)")
		return 2
	}
	logger := slog.New(slog.NewTextHandler(stderr, nil))
	srv, err := server.New(server.Config{
		Addr: *addr, DBPath: *db, Endpoint: *endpoint,
		OverlayCIDR: "10.100.0.0/24", KeyPath: *keyPath,
	}, server.NewMemStore(), logger)
	if err != nil {
		fmt.Fprintf(stderr, "server: %v\n", err)
		return 1
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := srv.Run(ctx); err != nil {
		fmt.Fprintf(stderr, "server: %v\n", err)
		return 1
	}
	return 0
}
