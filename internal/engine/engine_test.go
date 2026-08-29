package engine_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/schuettc/kempt/internal/engine"
	_ "github.com/schuettc/kempt/internal/engine/handlers"
	"github.com/schuettc/kempt/internal/machine"
	"github.com/schuettc/kempt/internal/manifest"
	"github.com/schuettc/kempt/internal/run"
)

const buildSrc = `
[kempt]
spec = 1

[packages.a]
description = "a"
  [[packages.a.symlink]]
  from = "src/a"
  to = "a"

[packages.b]
description = "b"
needs = ["a"]
  [[packages.b.install]]
  brew = { formulas = ["foo"] }
  [[packages.b.symlink]]
  from = "src/b"
  to = "b"
  only = { os = "windows" }

[packages.gc]
description = "git-clone fallback coverage"
  [[packages.gc.git-clone]]
  repo = "https://github.com/example/repo"
  to = "~/src/repo"

[packages.win]
description = "win only"
only = { os = "windows" }
  [[packages.win.symlink]]
  from = "src/w"
  to = "w"
`

func buildCtx(t *testing.T) *machine.Context {
	t.Helper()
	tmp := t.TempDir()
	return &machine.Context{
		Home:    tmp,
		RepoDir: tmp,
		OS:      "darwin",
		Arch:    "arm64",
		// Empty FakeRunner: LookPath("brew") misses, so the install step
		// resolves to OpBlocked "(brew not found)" deterministically.
		Runner: &run.FakeRunner{},
		Cache:  map[string]string{},
	}
}

func buildPlan(t *testing.T, ctx *machine.Context) *engine.Plan {
	t.Helper()
	m, findings := manifest.Parse([]byte(buildSrc))
	if len(findings) > 0 {
		t.Fatalf("parse findings: %v", findings)
	}
	findings = manifest.Validate(m)
	if len(findings) > 0 {
		t.Fatalf("validate findings: %v", findings)
	}
	pkgs, err := engine.Select(m, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := engine.BuildPlan(ctx, pkgs)
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func TestBuildPlanOrderAndOps(t *testing.T) {
	ctx := buildCtx(t)
	plan := buildPlan(t, ctx)

	if len(plan.Packages) != 4 {
		t.Fatalf("packages = %d, want 4", len(plan.Packages))
	}
	names := []string{plan.Packages[0].Name, plan.Packages[1].Name, plan.Packages[2].Name, plan.Packages[3].Name}
	wantNames := []string{"a", "b", "gc", "win"}
	for i, w := range wantNames {
		if names[i] != w {
			t.Fatalf("order = %v; want %v", names, wantNames)
		}
	}

	// a: single symlink change
	if got := plan.Packages[0].Steps[0].Delta.Op; got != engine.OpChange {
		t.Fatalf("a step op = %v, want change", got)
	}

	// b: install -> blocked (brew not found via empty FakeRunner), symlink -> skipped by only
	b := plan.Packages[1]
	if b.Steps[0].Delta.Op != engine.OpBlocked {
		t.Fatalf("b install op = %v, want blocked", b.Steps[0].Delta.Op)
	}
	if b.Steps[0].Delta.Detail != "install (brew not found)" {
		t.Fatalf("b install detail = %q", b.Steps[0].Delta.Detail)
	}
	if b.Steps[1].Delta.Op != engine.OpSkip {
		t.Fatalf("b symlink op = %v, want skip", b.Steps[1].Delta.Op)
	}

	// gc: git-clone step has no registered handler → engine fallback OpBlocked.
	// TODO(Task 6): this assertion moves to a different kind once git-clone is
	// implemented; re-point it at github-release or verify then.
	gc := plan.Packages[2]
	if gc.Steps[0].Delta.Op != engine.OpBlocked {
		t.Fatalf("gc git-clone op = %v, want blocked", gc.Steps[0].Delta.Op)
	}
	if gc.Steps[0].Delta.Detail != "handler not implemented yet (phase 1b)" {
		t.Fatalf("gc git-clone detail = %q", gc.Steps[0].Delta.Detail)
	}

	// win: whole package skipped
	if !plan.Packages[3].Skipped {
		t.Fatal("win package should be skipped")
	}
}

func TestRenderGolden(t *testing.T) {
	ctx := buildCtx(t)
	plan := buildPlan(t, ctx)

	var buf bytes.Buffer
	engine.Render(plan, &buf)

	want := fmt.Sprintf(`package a
  + symlink %s -> %s (create)
package b
  ! install (brew not found)
  - symlink (skipped: os != windows)
package gc
  ! handler not implemented yet (phase 1b)
package win (skipped: os != windows)
1 changes, 0 ok, 1 skipped, 2 blocked
software changes: 0, file changes: 1
`, ctx.Expand("a"), ctx.Expand("src/a"))

	if buf.String() != want {
		t.Fatalf("render mismatch\n--- got ---\n%s\n--- want ---\n%s", buf.String(), want)
	}
}
