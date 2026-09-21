package inventory

import (
	"testing"

	"github.com/schuettc/kempt/internal/machine"
	"github.com/schuettc/kempt/internal/run"
)

func ctxWith(fake *run.FakeRunner) *machine.Context {
	c, _ := machine.New("/r", fake)
	c.Home = "/h"
	return c
}

func TestPiInventoryTwoLineFormat(t *testing.T) {
	// Real `pi list` prints two lines per package: a spec line at the entry
	// indent (2 spaces) and a deeper-indented (4 spaces) resolved path. Only
	// the spec lines are entries; the resolved-path lines must be skipped.
	stdout := "User packages:\n" +
		"  npm:pi-quiet\n" +
		"    /Users/you/.pi/agent/npm/node_modules/pi-quiet\n" +
		"  git:github.com/obra/superpowers\n" +
		"    /Users/you/.pi/agent/git/github.com/obra/superpowers\n"
	c := ctxWith(&run.FakeRunner{Responses: map[string]run.Response{
		"pi list": {Stdout: stdout},
	}})
	inv, err := Pi(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(inv) != 2 {
		t.Fatalf("want exactly 2 entries, got %d: %v", len(inv), inv)
	}
	if _, ok := inv["npm:pi-quiet"]; !ok {
		t.Fatalf("missing npm:pi-quiet entry: %v", inv)
	}
	if _, ok := inv["git:github.com/obra/superpowers"]; !ok {
		t.Fatalf("missing git:github.com/obra/superpowers entry: %v", inv)
	}
	if _, ok := inv["/Users/you/.pi/agent/npm/node_modules/pi-quiet"]; ok {
		t.Fatal("resolved-path continuation line must not be an entry")
	}
	if _, ok := inv["/Users/you/.pi/agent/git/github.com/obra/superpowers"]; ok {
		t.Fatal("resolved-path continuation line must not be an entry")
	}
}

func TestPiInventorySkipsHeaders(t *testing.T) {
	c := ctxWith(&run.FakeRunner{Responses: map[string]run.Response{
		"pi list": {Stdout: "User packages:\n  npm:typescript@5.0.0\n  /Users/me/local-tool\n"},
	}})
	inv, err := Pi(c)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := inv["User packages:"]; ok {
		t.Fatal("header must not be an entry")
	}
	if inv["npm:typescript"] != "5.0.0" {
		t.Fatalf("typescript version = %q", inv["npm:typescript"])
	}
}
