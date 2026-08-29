package handlers

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/schuettc/kempt/internal/engine"
	"github.com/schuettc/kempt/internal/machine"
	"github.com/schuettc/kempt/internal/manifest"
	"github.com/schuettc/kempt/internal/release"
	"github.com/schuettc/kempt/internal/run"
)

func verifyCtx(t *testing.T, r run.Runner, rel release.Releases) *machine.Context {
	t.Helper()
	tmp := t.TempDir()
	return &machine.Context{
		Home:     tmp,
		RepoDir:  tmp,
		OS:       "darwin",
		Arch:     "arm64",
		Runner:   r,
		Releases: rel,
		Cache:    map[string]string{},
	}
}

func verifyHandler(t *testing.T) engine.Handler {
	t.Helper()
	h, ok := engine.HandlerFor("verify")
	if !ok {
		t.Fatal("verify handler not registered")
	}
	return h
}

func TestVerifyApplyNeverCalled(t *testing.T) {
	h := verifyHandler(t)
	if err := h.Apply(nil, manifest.VerifyStep{}); err == nil {
		t.Fatal("Apply should return an error")
	}
}

func TestVerifyCommandExistsPass(t *testing.T) {
	r := &run.FakeRunner{Responses: map[string]run.Response{
		"lookpath tool": {Stdout: "/usr/bin/tool"},
	}}
	ctx := verifyCtx(t, r, nil)
	h := verifyHandler(t)
	d, err := h.Inspect(ctx, manifest.VerifyStep{CommandExists: "tool"})
	if err != nil {
		t.Fatal(err)
	}
	if d.Op != engine.OpNoop {
		t.Fatalf("op = %v, want noop", d.Op)
	}
	if d.Detail != "verify: command tool ✓" {
		t.Fatalf("detail = %q", d.Detail)
	}
}

func TestVerifyCommandExistsFail(t *testing.T) {
	r := &run.FakeRunner{}
	ctx := verifyCtx(t, r, nil)
	h := verifyHandler(t)
	d, err := h.Inspect(ctx, manifest.VerifyStep{CommandExists: "tool"})
	if err != nil {
		t.Fatal(err)
	}
	if d.Op != engine.OpBlocked {
		t.Fatalf("op = %v, want blocked", d.Op)
	}
	if d.Detail != "verify: command tool MISSING" {
		t.Fatalf("detail = %q", d.Detail)
	}
}

func TestVerifySymlinkTargetPass(t *testing.T) {
	ctx := verifyCtx(t, &run.FakeRunner{}, nil)
	target := filepath.Join(ctx.RepoDir, "real")
	if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(ctx.Home, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	h := verifyHandler(t)
	d, err := h.Inspect(ctx, manifest.VerifyStep{SymlinkTarget: &manifest.SymlinkTargetCheck{
		Link: "~/link", Target: "real",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if d.Op != engine.OpNoop {
		t.Fatalf("op = %v, want noop; detail=%q", d.Op, d.Detail)
	}
	if !strings.Contains(d.Detail, link) || !strings.Contains(d.Detail, target) {
		t.Fatalf("detail missing both paths: %q", d.Detail)
	}
}

func TestVerifySymlinkTargetFail(t *testing.T) {
	ctx := verifyCtx(t, &run.FakeRunner{}, nil)
	link := filepath.Join(ctx.Home, "link")
	if err := os.Symlink(filepath.Join(ctx.RepoDir, "other"), link); err != nil {
		t.Fatal(err)
	}
	h := verifyHandler(t)
	d, err := h.Inspect(ctx, manifest.VerifyStep{SymlinkTarget: &manifest.SymlinkTargetCheck{
		Link: "~/link", Target: "real",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if d.Op != engine.OpBlocked {
		t.Fatalf("op = %v, want blocked", d.Op)
	}
	if !strings.Contains(d.Detail, ctx.Expand("~/link")) || !strings.Contains(d.Detail, ctx.Expand("real")) {
		t.Fatalf("detail missing both paths: %q", d.Detail)
	}
}

func TestVerifyVersionCurrentPass(t *testing.T) {
	r := &run.FakeRunner{Responses: map[string]run.Response{
		"tool --version": {Stdout: "tool version 1.2.3\n"},
	}}
	rel := release.FakeReleases{Tags: map[string]string{"o/r": "v1.2.3"}}
	ctx := verifyCtx(t, r, rel)
	h := verifyHandler(t)
	d, err := h.Inspect(ctx, manifest.VerifyStep{VersionCurrent: &manifest.VersionCheck{
		Repo: "o/r", Command: "tool --version",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if d.Op != engine.OpNoop {
		t.Fatalf("op = %v, want noop; detail=%q", d.Op, d.Detail)
	}
	if d.Detail != "verify: o/r current (v1.2.3)" {
		t.Fatalf("detail = %q", d.Detail)
	}
}

func TestVerifyVersionCurrentBehind(t *testing.T) {
	r := &run.FakeRunner{Responses: map[string]run.Response{
		"tool --version": {Stdout: "tool version 1.0.0\n"},
	}}
	rel := release.FakeReleases{Tags: map[string]string{"o/r": "v1.2.3"}}
	ctx := verifyCtx(t, r, rel)
	h := verifyHandler(t)
	d, err := h.Inspect(ctx, manifest.VerifyStep{VersionCurrent: &manifest.VersionCheck{
		Repo: "o/r", Command: "tool --version",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if d.Op != engine.OpBlocked {
		t.Fatalf("op = %v, want blocked", d.Op)
	}
	if d.Detail != "verify: o/r behind (latest v1.2.3)" {
		t.Fatalf("detail = %q", d.Detail)
	}
}

func TestVerifyVersionResolverError(t *testing.T) {
	r := &run.FakeRunner{Responses: map[string]run.Response{
		"tool --version": {Stdout: "1.2.3"},
	}}
	rel := release.FakeReleases{Tags: map[string]string{}} // no tag → resolver error
	ctx := verifyCtx(t, r, rel)
	h := verifyHandler(t)
	d, err := h.Inspect(ctx, manifest.VerifyStep{VersionCurrent: &manifest.VersionCheck{
		Repo: "o/r", Command: "tool --version",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if d.Op != engine.OpBlocked {
		t.Fatalf("op = %v, want blocked", d.Op)
	}
	if !strings.Contains(d.Detail, "cannot resolve latest:") {
		t.Fatalf("detail = %q", d.Detail)
	}
}

func TestVerifyVersionCommandError(t *testing.T) {
	r := &run.FakeRunner{Responses: map[string]run.Response{
		"tool --version": {Err: errors.New("boom")},
	}}
	rel := release.FakeReleases{Tags: map[string]string{"o/r": "v1.2.3"}}
	ctx := verifyCtx(t, r, rel)
	h := verifyHandler(t)
	d, err := h.Inspect(ctx, manifest.VerifyStep{VersionCurrent: &manifest.VersionCheck{
		Repo: "o/r", Command: "tool --version",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if d.Op != engine.OpBlocked {
		t.Fatalf("op = %v, want blocked", d.Op)
	}
	if !strings.Contains(d.Detail, "boom") {
		t.Fatalf("detail = %q", d.Detail)
	}
}

func TestVerifyMultiCheckJoinAndBlocked(t *testing.T) {
	r := &run.FakeRunner{Responses: map[string]run.Response{
		"lookpath tool": {Stdout: "/usr/bin/tool"},
	}}
	ctx := verifyCtx(t, r, nil)
	// command-exists passes; symlink-target fails (link does not exist).
	h := verifyHandler(t)
	d, err := h.Inspect(ctx, manifest.VerifyStep{
		CommandExists: "tool",
		SymlinkTarget: &manifest.SymlinkTargetCheck{Link: "~/missing", Target: "real"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if d.Op != engine.OpBlocked {
		t.Fatalf("op = %v, want blocked", d.Op)
	}
	if !strings.Contains(d.Detail, "; ") {
		t.Fatalf("detail should join with '; ': %q", d.Detail)
	}
	if !strings.Contains(d.Detail, "verify: command tool ✓") {
		t.Fatalf("detail missing passing check: %q", d.Detail)
	}
}

func TestVerifyMultiCheckAllPass(t *testing.T) {
	r := &run.FakeRunner{Responses: map[string]run.Response{
		"lookpath a": {Stdout: "/bin/a"},
		"lookpath b": {Stdout: "/bin/b"},
	}}
	ctx := verifyCtx(t, r, nil)
	h := verifyHandler(t)
	// Two command-exists checks cannot both live in one VerifyStep struct;
	// use command-exists + a passing symlink check.
	target := filepath.Join(ctx.RepoDir, "real")
	if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(ctx.Home, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	d, err := h.Inspect(ctx, manifest.VerifyStep{
		CommandExists: "a",
		SymlinkTarget: &manifest.SymlinkTargetCheck{Link: "~/link", Target: "real"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if d.Op != engine.OpNoop {
		t.Fatalf("op = %v, want noop; detail=%q", d.Op, d.Detail)
	}
	if !strings.Contains(d.Detail, "; ") {
		t.Fatalf("detail should join with '; ': %q", d.Detail)
	}
}
