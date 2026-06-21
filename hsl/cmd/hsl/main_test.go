package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestDispatchVersion(t *testing.T) {
	var out, errOut bytes.Buffer
	code := dispatch([]string{"version"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "hsl") {
		t.Fatalf("stdout = %q, want it to contain %q", out.String(), "hsl")
	}
}

func TestDispatchUnknown(t *testing.T) {
	var out, errOut bytes.Buffer
	code := dispatch([]string{"frobnicate"}, &out, &errOut)
	if code == 0 {
		t.Fatal("exit code = 0 for unknown subcommand, want non-zero")
	}
	if !strings.Contains(errOut.String(), "unknown subcommand") {
		t.Fatalf("stderr = %q, want it to mention unknown subcommand", errOut.String())
	}
}
