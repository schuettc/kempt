package handlers

import (
	"testing"

	"github.com/schuettc/kempt/internal/machine"
	"github.com/schuettc/kempt/internal/manifest"
	"github.com/schuettc/kempt/internal/run"
)

func extCtx(resp map[string]run.Response) *machine.Context {
	return &machine.Context{
		Runner: &run.FakeRunner{Responses: resp},
		Cache:  map[string]string{},
	}
}

func TestRollingExtensionsSelectsUnversioned(t *testing.T) {
	step := manifest.InstallStep{
		Pi:  []string{"npm:pi-creel", "npm:pi-quiet@0.2.0"},
		Npm: []string{"typescript", "@scope/x@1.0.0"},
	}
	got := RollingExtensions(step)
	if len(got) != 2 {
		t.Fatalf("want 2 rolling entries, got %d: %v", len(got), got)
	}
	if got[0] != (ExtEntry{Backend: "pi", Entry: "npm:pi-creel", Pkg: "pi-creel"}) {
		t.Errorf("pi entry = %+v", got[0])
	}
	if got[1] != (ExtEntry{Backend: "npm", Entry: "typescript", Pkg: "typescript"}) {
		t.Errorf("npm entry = %+v", got[1])
	}
}

func TestExtLatestReadsNpmView(t *testing.T) {
	ctx := extCtx(map[string]run.Response{
		"npm view pi-creel version": {Stdout: "0.1.1\n"},
	})
	v, err := ExtLatest(ctx, "pi-creel")
	if err != nil || v != "0.1.1" {
		t.Fatalf("ExtLatest = %q, %v; want 0.1.1, nil", v, err)
	}
}

func TestExtInstalledVersionFromPiList(t *testing.T) {
	ctx := extCtx(map[string]run.Response{
		"pi list": {Stdout: "User packages:\n  npm:pi-creel@0.1.0\n    /home/u/.pi/agent/npm/node_modules/pi-creel\n"},
	})
	v, known := ExtInstalledVersion(ctx, ExtEntry{Backend: "pi", Entry: "npm:pi-creel", Pkg: "pi-creel"})
	if !known || v != "0.1.0" {
		t.Fatalf("ExtInstalledVersion = %q, %v; want 0.1.0, true", v, known)
	}
}

func TestRollExtensionRunsPiInstallAndInvalidatesCache(t *testing.T) {
	fr := &run.FakeRunner{Responses: map[string]run.Response{
		"pi install npm:pi-creel": {Stdout: ""},
	}}
	ctx := &machine.Context{Runner: fr, Cache: map[string]string{piInventoryCmd: "stale"}}
	if err := RollExtension(ctx, ExtEntry{Backend: "pi", Entry: "npm:pi-creel", Pkg: "pi-creel"}); err != nil {
		t.Fatalf("RollExtension: %v", err)
	}
	if _, cached := ctx.Cache[piInventoryCmd]; cached {
		t.Error("pi inventory cache not invalidated")
	}
	if len(fr.Calls) != 1 || fr.Calls[0] != "pi install npm:pi-creel" {
		t.Errorf("calls = %v", fr.Calls)
	}
}
