package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/schuettc/kempt/internal/machine"
	"github.com/schuettc/kempt/internal/release"
	"github.com/schuettc/kempt/internal/run"
)

// withContext overrides newContext for the duration of a test.
func withContext(t *testing.T, r run.Runner, rel release.Releases) {
	t.Helper()
	orig := newContext
	newContext = func(repoDir string) (*machine.Context, error) {
		return &machine.Context{
			Home:     t.TempDir(),
			RepoDir:  repoDir,
			OS:       "darwin",
			Arch:     "arm64",
			Runner:   r,
			Releases: rel,
			Cache:    map[string]string{},
		}, nil
	}
	t.Cleanup(func() { newContext = orig })
}

const verifySrc = `
[kempt]
spec = 1

[packages.a]
description = "a"
  [[packages.a.verify]]
  command-exists = "have"
  [[packages.a.verify]]
  command-exists = "missing"
`

func TestVerifyCommandPassAndFail(t *testing.T) {
	withContext(t, &run.FakeRunner{Responses: map[string]run.Response{
		"lookpath have": {Stdout: "/bin/have"},
	}}, nil)
	p := writeTemp(t, verifySrc)
	var out, errw bytes.Buffer
	code := Dispatch([]string{"verify", "-manifest", p}, &out, &errw)
	if code != 1 {
		t.Fatalf("exit = %d, want 1; stderr=%s", code, errw.String())
	}
	s := out.String()
	if !strings.Contains(s, "verify: command have ✓") {
		t.Fatalf("stdout missing pass line: %q", s)
	}
	if !strings.Contains(s, "verify: command missing MISSING") {
		t.Fatalf("stdout missing fail line: %q", s)
	}
	if !strings.Contains(s, "1 passed, 1 failed") {
		t.Fatalf("stdout missing summary: %q", s)
	}
}

func TestVerifyCommandAllPass(t *testing.T) {
	withContext(t, &run.FakeRunner{Responses: map[string]run.Response{
		"lookpath have":    {Stdout: "/bin/have"},
		"lookpath missing": {Stdout: "/bin/missing"},
	}}, nil)
	p := writeTemp(t, verifySrc)
	var out, errw bytes.Buffer
	code := Dispatch([]string{"verify", "-manifest", p}, &out, &errw)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%s", code, errw.String())
	}
	if !strings.Contains(out.String(), "2 passed, 0 failed") {
		t.Fatalf("stdout missing summary: %q", out.String())
	}
}

func TestVerifyNoVerifySteps(t *testing.T) {
	withContext(t, &run.FakeRunner{}, nil)
	p := writeTemp(t, "[kempt]\nspec = 1\n[packages.a]\ndescription = \"a\"\n  [[packages.a.symlink]]\n  from = \"src/a\"\n  to = \"a\"\n")
	var out, errw bytes.Buffer
	code := Dispatch([]string{"verify", "-manifest", p}, &out, &errw)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%s", code, errw.String())
	}
	if !strings.Contains(out.String(), "no verify steps") {
		t.Fatalf("stdout = %q", out.String())
	}
}
