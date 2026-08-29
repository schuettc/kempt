package handlers

import (
	"errors"
	"reflect"
	"testing"

	"github.com/schuettc/kempt/internal/engine"
	"github.com/schuettc/kempt/internal/machine"
	"github.com/schuettc/kempt/internal/manifest"
	"github.com/schuettc/kempt/internal/run"
)

func getInstallHandler(t *testing.T) engine.Handler {
	t.Helper()
	h, ok := engine.HandlerFor("install")
	if !ok {
		t.Fatal("install handler not registered")
	}
	return h
}

// installCtx builds a Context wired to the given FakeRunner. OS defaults to
// darwin; callers override via the returned Context.
func installCtx(t *testing.T, os string, fake *run.FakeRunner) *machine.Context {
	t.Helper()
	return &machine.Context{
		Home:    t.TempDir(),
		RepoDir: t.TempDir(),
		OS:      os,
		Arch:    "arm64",
		Runner:  fake,
		Cache:   map[string]string{},
	}
}

// brewFound scripts the LookPath hit plus inventory outputs.
func brewFound(formulas, casks, taps string) map[string]run.Response {
	return map[string]run.Response{
		"lookpath brew":          {Stdout: "/opt/homebrew/bin/brew"},
		"brew list --formula -1": {Stdout: formulas},
		"brew list --cask -1":    {Stdout: casks},
		"brew tap":               {Stdout: taps},
	}
}

func TestInstallInspectAllPresent(t *testing.T) {
	fake := &run.FakeRunner{Responses: brewFound("jq\nripgrep\n", "ghostty\n", "homebrew/cask\n")}
	ctx := installCtx(t, "darwin", fake)
	h := getInstallHandler(t)
	step := manifest.InstallStep{Brew: &manifest.BrewSpec{
		Formulas: []string{"jq", "ripgrep"},
		Casks:    []string{"ghostty"},
		Taps:     []string{"homebrew/cask"},
	}}

	d, err := h.Inspect(ctx, step)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if d.Op != engine.OpNoop {
		t.Fatalf("op = %v; want OpNoop", d.Op)
	}
	if d.Detail != "brew: 4 present" {
		t.Fatalf("detail = %q; want %q", d.Detail, "brew: 4 present")
	}
	// Only inventory commands (plus lookpath) may have run.
	wantCalls := []string{
		"lookpath brew",
		"brew list --formula -1",
		"brew list --cask -1",
		"brew tap",
	}
	if !reflect.DeepEqual(fake.Calls, wantCalls) {
		t.Fatalf("calls = %v; want %v", fake.Calls, wantCalls)
	}
}

func TestInstallInspectMissing(t *testing.T) {
	fake := &run.FakeRunner{Responses: brewFound("ripgrep\n", "", "")}
	ctx := installCtx(t, "darwin", fake)
	h := getInstallHandler(t)
	step := manifest.InstallStep{Brew: &manifest.BrewSpec{
		Formulas: []string{"jq", "ripgrep", "fd"},
		Casks:    []string{"ghostty"},
		Taps:     []string{"x/y"},
	}}

	d, err := h.Inspect(ctx, step)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if d.Op != engine.OpChange {
		t.Fatalf("op = %v; want OpChange", d.Op)
	}
	want := "brew install: fd jq; casks: ghostty; taps: x/y"
	if d.Detail != want {
		t.Fatalf("detail = %q; want %q", d.Detail, want)
	}
}

func TestInstallInspectMissingFormulasOnly(t *testing.T) {
	fake := &run.FakeRunner{Responses: brewFound("", "ghostty\n", "x/y\n")}
	ctx := installCtx(t, "darwin", fake)
	h := getInstallHandler(t)
	step := manifest.InstallStep{Brew: &manifest.BrewSpec{
		Formulas: []string{"jq"},
		Casks:    []string{"ghostty"},
		Taps:     []string{"x/y"},
	}}

	d, err := h.Inspect(ctx, step)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if d.Detail != "brew install: jq" {
		t.Fatalf("detail = %q; want %q", d.Detail, "brew install: jq")
	}
}

func TestInstallInspectMemoizesCache(t *testing.T) {
	fake := &run.FakeRunner{Responses: brewFound("jq\n", "", "")}
	ctx := installCtx(t, "darwin", fake)
	h := getInstallHandler(t)
	step := manifest.InstallStep{Brew: &manifest.BrewSpec{Formulas: []string{"jq"}}}

	if _, err := h.Inspect(ctx, step); err != nil {
		t.Fatalf("Inspect 1: %v", err)
	}
	if _, err := h.Inspect(ctx, step); err != nil {
		t.Fatalf("Inspect 2: %v", err)
	}
	// Each inventory command should have run at most once.
	counts := map[string]int{}
	for _, c := range fake.Calls {
		counts[c]++
	}
	if counts["brew list --formula -1"] != 1 {
		t.Fatalf("formula inventory ran %d times; want 1", counts["brew list --formula -1"])
	}
}

func TestInstallInventoryError(t *testing.T) {
	resp := brewFound("", "", "")
	resp["brew list --formula -1"] = run.Response{Err: errors.New("brew broke")}
	fake := &run.FakeRunner{Responses: resp}
	ctx := installCtx(t, "darwin", fake)
	h := getInstallHandler(t)
	step := manifest.InstallStep{Brew: &manifest.BrewSpec{Formulas: []string{"jq"}}}

	if _, err := h.Inspect(ctx, step); err == nil {
		t.Fatal("expected error from failing inventory command")
	}
}

func TestInstallBrewNotFound(t *testing.T) {
	fake := &run.FakeRunner{Responses: map[string]run.Response{}}
	ctx := installCtx(t, "darwin", fake)
	h := getInstallHandler(t)
	step := manifest.InstallStep{Brew: &manifest.BrewSpec{Formulas: []string{"jq"}}}

	d, err := h.Inspect(ctx, step)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if d.Op != engine.OpBlocked {
		t.Fatalf("op = %v; want OpBlocked", d.Op)
	}
	if d.Detail != "install (brew not found)" {
		t.Fatalf("detail = %q; want %q", d.Detail, "install (brew not found)")
	}
}

func TestInstallAptNotImplemented(t *testing.T) {
	fake := &run.FakeRunner{Responses: map[string]run.Response{}}
	ctx := installCtx(t, "linux", fake)
	h := getInstallHandler(t)
	step := manifest.InstallStep{Apt: []string{"jq"}}

	d, err := h.Inspect(ctx, step)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if d.Op != engine.OpBlocked {
		t.Fatalf("op = %v; want OpBlocked", d.Op)
	}
	if d.Detail != "install (backend apt not implemented in this phase)" {
		t.Fatalf("detail = %q", d.Detail)
	}
}

func TestInstallWingetOnDarwinSkips(t *testing.T) {
	fake := &run.FakeRunner{Responses: map[string]run.Response{}}
	ctx := installCtx(t, "darwin", fake)
	h := getInstallHandler(t)
	step := manifest.InstallStep{Winget: []string{"jq"}}

	d, err := h.Inspect(ctx, step)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if d.Op != engine.OpSkip {
		t.Fatalf("op = %v; want OpSkip", d.Op)
	}
	if d.Detail != "install (no backend for darwin)" {
		t.Fatalf("detail = %q", d.Detail)
	}
}

func TestInstallLinuxBrewWins(t *testing.T) {
	fake := &run.FakeRunner{Responses: brewFound("jq\n", "", "")}
	ctx := installCtx(t, "linux", fake)
	h := getInstallHandler(t)
	step := manifest.InstallStep{
		Brew: &manifest.BrewSpec{Formulas: []string{"jq"}},
		Apt:  []string{"ripgrep"},
	}

	d, err := h.Inspect(ctx, step)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if d.Op != engine.OpNoop {
		t.Fatalf("op = %v; want OpNoop (brew wins)", d.Op)
	}
}

func TestInstallApplyOrderAndCacheClear(t *testing.T) {
	resp := brewFound("ripgrep\n", "", "")
	resp["brew tap x/y"] = run.Response{}
	resp["brew install fd jq"] = run.Response{}
	resp["brew install --cask ghostty"] = run.Response{}
	fake := &run.FakeRunner{Responses: resp}
	ctx := installCtx(t, "darwin", fake)
	h := getInstallHandler(t)
	step := manifest.InstallStep{Brew: &manifest.BrewSpec{
		Formulas: []string{"jq", "ripgrep", "fd"},
		Casks:    []string{"ghostty"},
		Taps:     []string{"x/y"},
	}}

	// Inspect first to populate cache (mirrors BuildPlan → Execute).
	if _, err := h.Inspect(ctx, step); err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	// Prime cache keys so we can prove Apply deletes them.
	ctx.Cache["brew list --formula -1"] = "ripgrep\n"

	fake.Calls = nil
	if err := h.Apply(ctx, step); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	want := []string{
		"brew tap x/y",
		"brew install fd jq",
		"brew install --cask ghostty",
	}
	if !reflect.DeepEqual(fake.Calls, want) {
		t.Fatalf("apply calls = %v; want %v", fake.Calls, want)
	}
	for _, k := range []string{"brew list --formula -1", "brew list --cask -1", "brew tap"} {
		if _, ok := ctx.Cache[k]; ok {
			t.Fatalf("cache key %q not cleared after Apply", k)
		}
	}
}

func TestInstallApplySkipsEmptyGroups(t *testing.T) {
	resp := brewFound("", "", "")
	resp["brew install jq"] = run.Response{}
	fake := &run.FakeRunner{Responses: resp}
	ctx := installCtx(t, "darwin", fake)
	h := getInstallHandler(t)
	step := manifest.InstallStep{Brew: &manifest.BrewSpec{Formulas: []string{"jq"}}}

	if _, err := h.Inspect(ctx, step); err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	fake.Calls = nil
	if err := h.Apply(ctx, step); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	want := []string{"brew install jq"}
	if !reflect.DeepEqual(fake.Calls, want) {
		t.Fatalf("apply calls = %v; want %v", fake.Calls, want)
	}
}
