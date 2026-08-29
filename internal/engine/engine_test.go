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
		Runner:  run.RealRunner{},
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

	if len(plan.Packages) != 3 {
		t.Fatalf("packages = %d, want 3", len(plan.Packages))
	}
	if plan.Packages[0].Name != "a" || plan.Packages[1].Name != "b" || plan.Packages[2].Name != "win" {
		t.Fatalf("order = %v", []string{plan.Packages[0].Name, plan.Packages[1].Name, plan.Packages[2].Name})
	}

	// a: single symlink change
	if got := plan.Packages[0].Steps[0].Delta.Op; got != engine.OpChange {
		t.Fatalf("a step op = %v, want change", got)
	}

	// b: install -> blocked (no handler), symlink -> skipped by only
	b := plan.Packages[1]
	if b.Steps[0].Delta.Op != engine.OpBlocked {
		t.Fatalf("b install op = %v, want blocked", b.Steps[0].Delta.Op)
	}
	if b.Steps[0].Delta.Detail != "handler not implemented yet (phase 1b)" {
		t.Fatalf("b install detail = %q", b.Steps[0].Delta.Detail)
	}
	if b.Steps[1].Delta.Op != engine.OpSkip {
		t.Fatalf("b symlink op = %v, want skip", b.Steps[1].Delta.Op)
	}

	// win: whole package skipped
	if !plan.Packages[2].Skipped {
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
  ! handler not implemented yet (phase 1b)
  - symlink (skipped: os != windows)
package win (skipped: os != windows)
1 changes, 0 ok, 1 skipped, 1 blocked
software changes: 0, file changes: 1
`, ctx.Expand("a"), ctx.Expand("src/a"))

	if buf.String() != want {
		t.Fatalf("render mismatch\n--- got ---\n%s\n--- want ---\n%s", buf.String(), want)
	}
}
