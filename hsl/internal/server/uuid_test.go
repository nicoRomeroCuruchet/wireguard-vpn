package server

import (
	"regexp"
	"testing"
)

func TestNewUUIDFormat(t *testing.T) {
	id, err := newUUID()
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	if !re.MatchString(id) {
		t.Fatalf("uuid %q not v4 format", id)
	}
}
