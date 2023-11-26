package spfmacro

import (
	"testing"

	"github.com/coredns/caddy"
)

var (
	spfMacro1 = `spfmacro txt:gmail.com`
)

// TestSetup tests the various things that should be parsed by setup.
// Make sure you also test for parse errors.
func TestSetup(t *testing.T) {
	c := caddy.NewTestController("dns", spfMacro1)
	if err := setup(c); err != nil {
		t.Fatalf("Expected no errors, but got: %v", err)
	}

	c = caddy.NewTestController("dns", `spfmacro txt:gmail.com`)
	if err := setup(c); err == nil {
		t.Fatalf("Expected errors, but got: %v", err)
	}
}
