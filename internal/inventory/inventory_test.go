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
