package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/nromero/hsl/internal/client"
	"github.com/nromero/hsl/internal/server"
)

// stringSlice is a flag.Value that accumulates repeated flag occurrences.
type stringSlice []string

func (s *stringSlice) String() string { return strings.Join(*s, ",") }
func (s *stringSlice) Set(v string) error {
	*s = append(*s, v)
	return nil
}

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
		return runClient(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown subcommand %q\n", args[0])
		return 2
	}
}

func runClient(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: hsl client <register|run> [flags]")
		return 2
	}
	switch args[0] {
	case "register":
		fs := flag.NewFlagSet("client register", flag.ContinueOnError)
		fs.SetOutput(stderr)
		serverURL := fs.String("server", "", "control plane URL, e.g. http://1.2.3.4:8080 (required)")
		hostname := fs.String("hostname", "", "node hostname (default: OS hostname)")
		stateDir := fs.String("state-dir", "", "state directory (default: ~/.local/state/hsl)")
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}
		if *serverURL == "" {
			fmt.Fprintln(stderr, "client register: --server is required")
			return 2
		}
		st, err := client.Register(*serverURL, *hostname, *stateDir)
		if err != nil {
			fmt.Fprintf(stderr, "client register: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "registered: node_id=%s overlay_ip=%s\n", st.NodeID, st.OverlayIP)
		return 0
	case "run":
		fs := flag.NewFlagSet("client run", flag.ContinueOnError)
		fs.SetOutput(stderr)
		serverURL := fs.String("server", "", "control plane URL (required)")
		stateDir := fs.String("state-dir", "", "state directory (default: ~/.local/state/hsl)")
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}
		if *serverURL == "" {
			fmt.Fprintln(stderr, "client run: --server is required")
			return 2
		}
		logger := slog.New(slog.NewTextHandler(stderr, nil))
		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()
		if err := client.Run(ctx, *serverURL, *stateDir, logger); err != nil {
			fmt.Fprintf(stderr, "client run: %v\n", err)
			return 1
		}
		return 0
	default:
		fmt.Fprintf(stderr, "unknown client subcommand %q\n", args[0])
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
	var advertiseRoutes stringSlice
	fs.Var(&advertiseRoutes, "advertise-routes", "CIDR to advertise to clients (repeatable)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *endpoint == "" {
		fmt.Fprintln(stderr, "server: --endpoint is required (e.g. 1.2.3.4:51820)")
		return 2
	}
	if err := os.MkdirAll(filepath.Dir(*db), 0o700); err != nil {
		fmt.Fprintf(stderr, "server: %v\n", err)
		return 1
	}
	store, err := server.NewSQLiteStore(*db)
	if err != nil {
		fmt.Fprintf(stderr, "server: open db: %v\n", err)
		return 1
	}
	defer store.Close()
	logger := slog.New(slog.NewTextHandler(stderr, nil))
	srv, err := server.New(server.Config{
		Addr: *addr, DBPath: *db, Endpoint: *endpoint,
		OverlayCIDR: "10.100.0.0/24", KeyPath: *keyPath,
		AdvertisedRoutes: advertiseRoutes,
	}, store, logger)
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
