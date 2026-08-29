package handlers

import (
	"errors"
	"reflect"
	"strings"
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

func TestInstallInspectMissingCasksOnly(t *testing.T) {
	// No formulas missing; only casks absent — Detail must not have a stray semicolon.
	fake := &run.FakeRunner{Responses: brewFound("jq\n", "", "")}
	ctx := installCtx(t, "darwin", fake)
	h := getInstallHandler(t)
	step := manifest.InstallStep{Brew: &manifest.BrewSpec{
		Formulas: []string{"jq"},
		Casks:    []string{"ghostty"},
	}}

	d, err := h.Inspect(ctx, step)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if d.Op != engine.OpChange {
		t.Fatalf("op = %v; want OpChange", d.Op)
	}
	want := "brew install: casks: ghostty"
	if d.Detail != want {
		t.Fatalf("detail = %q; want %q", d.Detail, want)
	}
}

func TestInstallInspectMissingTapsOnly(t *testing.T) {
	// No formulas or casks missing; only taps absent.
	fake := &run.FakeRunner{Responses: brewFound("jq\n", "ghostty\n", "")}
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
	if d.Op != engine.OpChange {
		t.Fatalf("op = %v; want OpChange", d.Op)
	}
	want := "brew install: taps: x/y"
	if d.Detail != want {
		t.Fatalf("detail = %q; want %q", d.Detail, want)
	}
}

func TestInstallWingetOnWindowsBlocked(t *testing.T) {
	// Windows + winget content → OpBlocked (winget not implemented this phase).
	fake := &run.FakeRunner{Responses: map[string]run.Response{}}
	ctx := installCtx(t, "windows", fake)
	h := getInstallHandler(t)
	step := manifest.InstallStep{Winget: []string{"Microsoft.WindowsTerminal"}}

	d, err := h.Inspect(ctx, step)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if d.Op != engine.OpBlocked {
		t.Fatalf("op = %v; want OpBlocked", d.Op)
	}
	want := "install (backend winget not implemented in this phase)"
	if d.Detail != want {
		t.Fatalf("detail = %q; want %q", d.Detail, want)
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

// --- npm backend ---

const npmInvCmd = "npm ls -g --depth=0 --parseable"

func TestInstallNpmAllPresent(t *testing.T) {
	fake := &run.FakeRunner{Responses: map[string]run.Response{
		"lookpath npm": {Stdout: "/usr/bin/npm"},
		npmInvCmd:      {Stdout: "/usr/lib\n/usr/lib/node_modules/typescript\n/usr/lib/node_modules/prettier\n"},
	}}
	ctx := installCtx(t, "darwin", fake)
	h := getInstallHandler(t)
	step := manifest.InstallStep{Npm: []string{"typescript", "prettier"}}

	d, err := h.Inspect(ctx, step)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if d.Op != engine.OpNoop {
		t.Fatalf("op = %v; want OpNoop", d.Op)
	}
	if d.Detail != "npm: 2 present" {
		t.Fatalf("detail = %q; want %q", d.Detail, "npm: 2 present")
	}
	wantCalls := []string{"lookpath npm", npmInvCmd}
	if !reflect.DeepEqual(fake.Calls, wantCalls) {
		t.Fatalf("calls = %v; want %v", fake.Calls, wantCalls)
	}
}

func TestInstallNpmMissing(t *testing.T) {
	fake := &run.FakeRunner{Responses: map[string]run.Response{
		"lookpath npm":            {Stdout: "/usr/bin/npm"},
		npmInvCmd:                 {Stdout: "/usr/lib\n/usr/lib/node_modules/typescript\n"},
		"npm install -g prettier": {},
	}}
	ctx := installCtx(t, "darwin", fake)
	h := getInstallHandler(t)
	step := manifest.InstallStep{Npm: []string{"typescript", "prettier"}}

	d, err := h.Inspect(ctx, step)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if d.Op != engine.OpChange {
		t.Fatalf("op = %v; want OpChange", d.Op)
	}
	if d.Detail != "npm install: prettier" {
		t.Fatalf("detail = %q; want %q", d.Detail, "npm install: prettier")
	}

	fake.Calls = nil
	if err := h.Apply(ctx, step); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	// LookPath guard in npmApply now precedes the install command.
	want := []string{"lookpath npm", "npm install -g prettier"}
	if !reflect.DeepEqual(fake.Calls, want) {
		t.Fatalf("apply calls = %v; want %v", fake.Calls, want)
	}
	if _, ok := ctx.Cache[npmInvCmd]; ok {
		t.Fatalf("npm cache key not cleared after Apply")
	}
}

func TestInstallNpmNotFound(t *testing.T) {
	fake := &run.FakeRunner{Responses: map[string]run.Response{}}
	ctx := installCtx(t, "darwin", fake)
	h := getInstallHandler(t)
	step := manifest.InstallStep{Npm: []string{"typescript"}}

	d, err := h.Inspect(ctx, step)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if d.Op != engine.OpBlocked {
		t.Fatalf("op = %v; want OpBlocked", d.Op)
	}
	if d.Detail != "install (npm not found)" {
		t.Fatalf("detail = %q; want %q", d.Detail, "install (npm not found)")
	}
}

// --- pi backend ---

func TestInstallPiAllPresent(t *testing.T) {
	fake := &run.FakeRunner{Responses: map[string]run.Response{
		"lookpath pi": {Stdout: "/usr/bin/pi"},
		"pi list":     {Stdout: "npm:typescript@5.0.0\n/Users/me/local-tool\n"},
	}}
	ctx := installCtx(t, "darwin", fake)
	h := getInstallHandler(t)
	step := manifest.InstallStep{Pi: []string{"npm:typescript", "/Users/me/local-tool"}}

	d, err := h.Inspect(ctx, step)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if d.Op != engine.OpNoop {
		t.Fatalf("op = %v; want OpNoop", d.Op)
	}
	if d.Detail != "pi: 2 present" {
		t.Fatalf("detail = %q; want %q", d.Detail, "pi: 2 present")
	}
	wantCalls := []string{"lookpath pi", "pi list"}
	if !reflect.DeepEqual(fake.Calls, wantCalls) {
		t.Fatalf("calls = %v; want %v", fake.Calls, wantCalls)
	}
}

func TestInstallPiMissing(t *testing.T) {
	fake := &run.FakeRunner{Responses: map[string]run.Response{
		"lookpath pi":           {Stdout: "/usr/bin/pi"},
		"pi list":               {Stdout: "npm:typescript@5.0.0\n"},
		"pi install npm:eslint": {},
	}}
	ctx := installCtx(t, "darwin", fake)
	h := getInstallHandler(t)
	step := manifest.InstallStep{Pi: []string{"npm:typescript", "npm:eslint"}}

	d, err := h.Inspect(ctx, step)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if d.Op != engine.OpChange {
		t.Fatalf("op = %v; want OpChange", d.Op)
	}
	if d.Detail != "pi install: npm:eslint" {
		t.Fatalf("detail = %q; want %q", d.Detail, "pi install: npm:eslint")
	}

	fake.Calls = nil
	if err := h.Apply(ctx, step); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	// LookPath guard in piApply now precedes the install command.
	want := []string{"lookpath pi", "pi install npm:eslint"}
	if !reflect.DeepEqual(fake.Calls, want) {
		t.Fatalf("apply calls = %v; want %v", fake.Calls, want)
	}
	if _, ok := ctx.Cache["pi list"]; ok {
		t.Fatalf("pi cache key not cleared after Apply")
	}
}

func TestInstallPiNotFound(t *testing.T) {
	fake := &run.FakeRunner{Responses: map[string]run.Response{}}
	ctx := installCtx(t, "darwin", fake)
	h := getInstallHandler(t)
	step := manifest.InstallStep{Pi: []string{"npm:typescript"}}

	d, err := h.Inspect(ctx, step)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if d.Op != engine.OpBlocked {
		t.Fatalf("op = %v; want OpBlocked", d.Op)
	}
	if d.Detail != "install (pi not found)" {
		t.Fatalf("detail = %q; want %q", d.Detail, "install (pi not found)")
	}
}

// --- combined brew + npm (additive) ---

func TestInstallCombinedBrewNpm(t *testing.T) {
	resp := brewFound("jq\n", "", "")
	resp["brew install fd"] = run.Response{}
	resp["lookpath npm"] = run.Response{Stdout: "/usr/bin/npm"}
	resp[npmInvCmd] = run.Response{Stdout: "/usr/lib\n/usr/lib/node_modules/typescript\n"}
	resp["npm install -g prettier"] = run.Response{}
	fake := &run.FakeRunner{Responses: resp}
	ctx := installCtx(t, "darwin", fake)
	h := getInstallHandler(t)
	step := manifest.InstallStep{
		Brew: &manifest.BrewSpec{Formulas: []string{"jq", "fd"}},
		Npm:  []string{"typescript", "prettier"},
	}

	d, err := h.Inspect(ctx, step)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if d.Op != engine.OpChange {
		t.Fatalf("op = %v; want OpChange", d.Op)
	}
	want := "brew install: fd; npm install: prettier"
	if d.Detail != want {
		t.Fatalf("detail = %q; want %q", d.Detail, want)
	}

	fake.Calls = nil
	if err := h.Apply(ctx, step); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	// LookPath guard in npmApply now precedes the install command.
	wantCalls := []string{"brew install fd", "lookpath npm", "npm install -g prettier"}
	if !reflect.DeepEqual(fake.Calls, wantCalls) {
		t.Fatalf("apply calls = %v; want %v", fake.Calls, wantCalls)
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

// TestInstallCombinedBrewChangeNpmAbsent verifies finding #1: when brew has
// changes and npm is absent, Inspect returns OpChange (brew segment) with npm
// blocked in the Detail; Apply issues the brew install and does NOT error on
// npm, and no npm commands are recorded.
func TestInstallCombinedBrewChangeNpmAbsent(t *testing.T) {
	resp := brewFound("jq\n", "", "") // fd missing → brew OpChange
	resp["brew install fd"] = run.Response{}
	// npm absent: no "lookpath npm" response → LookPath returns error
	fake := &run.FakeRunner{Responses: resp}
	ctx := installCtx(t, "darwin", fake)
	h := getInstallHandler(t)
	step := manifest.InstallStep{
		Brew: &manifest.BrewSpec{Formulas: []string{"jq", "fd"}},
		Npm:  []string{"prettier"},
	}

	d, err := h.Inspect(ctx, step)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	// Combined: brew=OpChange, npm=OpBlocked → overall OpChange.
	if d.Op != engine.OpChange {
		t.Fatalf("op = %v; want OpChange", d.Op)
	}

	fake.Calls = nil
	if err := h.Apply(ctx, step); err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	// LookPath guard fires but returns nil; no npm ls or npm install must run.
	for _, c := range fake.Calls {
		if c == npmInvCmd || strings.HasPrefix(c, "npm install") {
			t.Fatalf("unexpected npm inventory/install call after brew-only apply: %q", c)
		}
	}
	if len(fake.Calls) == 0 {
		t.Fatal("expected brew install call but got none")
	}
}

// TestInstallPiHeaderTolerated verifies finding #2: section-header lines in
// "pi list" output (e.g. "User packages:") are silently skipped so they are
// not mistaken for package identifiers.
func TestInstallPiHeaderTolerated(t *testing.T) {
	fake := &run.FakeRunner{Responses: map[string]run.Response{
		"lookpath pi": {Stdout: "/usr/bin/pi"},
		// Fixture with a real-world section header.
		"pi list": {Stdout: "User packages:\n  npm:typescript@5.0.0\n  /Users/me/local-tool\n"},
	}}
	ctx := installCtx(t, "darwin", fake)
	h := getInstallHandler(t)
	step := manifest.InstallStep{Pi: []string{"npm:typescript", "/Users/me/local-tool"}}

	d, err := h.Inspect(ctx, step)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if d.Op != engine.OpNoop {
		t.Fatalf("op = %v; want OpNoop (header must be ignored)", d.Op)
	}
	if d.Detail != "pi: 2 present" {
		t.Fatalf("detail = %q; want \"pi: 2 present\"", d.Detail)
	}
}

// TestInstallNpmScopedPackage verifies finding #3: the npm inventory parser
// preserves @scope/name for scoped packages (the entire segment after
// node_modules/, NOT just the basename), so a scoped desired entry matches.
func TestInstallNpmScopedPackage(t *testing.T) {
	fake := &run.FakeRunner{Responses: map[string]run.Response{
		"lookpath npm": {Stdout: "/usr/bin/npm"},
		// Scoped package path: the name is the two-segment @scope/pkg part.
		npmInvCmd: {Stdout: "/usr/lib\n/usr/lib/node_modules/@earendil-works/pi-coding-agent\n"},
	}}
	ctx := installCtx(t, "darwin", fake)
	h := getInstallHandler(t)
	step := manifest.InstallStep{Npm: []string{"@earendil-works/pi-coding-agent"}}

	d, err := h.Inspect(ctx, step)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if d.Op != engine.OpNoop {
		t.Fatalf("op = %v; want OpNoop (scoped package must be recognized)", d.Op)
	}
	if d.Detail != "npm: 1 present" {
		t.Fatalf("detail = %q; want \"npm: 1 present\"", d.Detail)
	}
}
