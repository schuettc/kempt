package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/schuettc/kempt/internal/machine"
	"github.com/schuettc/kempt/internal/release"
	"github.com/schuettc/kempt/internal/run"
	"github.com/schuettc/kempt/internal/state"
	"github.com/schuettc/kempt/internal/version"
)

const updateManifest = `
[kempt]
spec = 1

[packages.a]
description = "a"
  [[packages.a.symlink]]
  from = "src/rc"
  to = "~/.rc"
`

// setupUpdate wires loadState, newContext (FakeRunner+FakeReleases, tempdir
// Home), and osExecutable at a tempdir fake exe. It returns (home, repo, exe).
func setupUpdate(t *testing.T, r *run.FakeRunner, rel release.Releases) (home, repo, exe string) {
	t.Helper()
	repo = t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "kempt.toml"), []byte(updateManifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "src", "rc"), []byte("rc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	home = t.TempDir()

	exeDir := t.TempDir()
	exe = filepath.Join(exeDir, "kempt")
	if err := os.WriteFile(exe, []byte("OLD"), 0o755); err != nil {
		t.Fatal(err)
	}

	stateStore := &state.Store{Dir: t.TempDir()}
	if err := stateStore.Save(&state.State{RepoDir: repo, Packages: []string{"a"}}); err != nil {
		t.Fatal(err)
	}

	origLoad, origCtx, origExe := loadState, newContext, osExecutable
	loadState = func() (*state.State, bool, error) { return stateStore.Load() }
	newContext = func(repoDir string) (*machine.Context, error) {
		return &machine.Context{
			Home:     home,
			RepoDir:  repoDir,
			OS:       "darwin",
			Arch:     "arm64",
			Runner:   r,
			Releases: rel,
			Cache:    map[string]string{},
		}, nil
	}
	osExecutable = func() (string, error) { return exe, nil }
	t.Cleanup(func() {
		loadState, newContext, osExecutable = origLoad, origCtx, origExe
	})
	return home, repo, exe
}

// TestUpdateCurrentBinaryConverges: pull succeeds, the release tag matches the
// current version (vdev → "dev") so self-update is a no-op, and a files-only
// manifest converges. Exit 0.
func TestUpdateCurrentBinaryConverges(t *testing.T) {
	r := &run.FakeRunner{}
	rel := release.FakeReleases{Tags: map[string]string{"schuettc/kempt": "v" + version.Number()}}
	home, repo, exe := setupUpdate(t, r, rel)
	r.Responses = map[string]run.Response{
		"git -C " + repo + " pull --rebase --autostash": {Stdout: ""},
	}

	var out, errw bytes.Buffer
	code := Dispatch([]string{"update"}, &out, &errw)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; out=%s err=%s", code, out.String(), errw.String())
	}
	// Files converged: symlink created.
	link := filepath.Join(home, ".rc")
	if target, err := os.Readlink(link); err != nil {
		t.Fatalf("symlink not created: %v", err)
	} else if want := filepath.Join(repo, "src", "rc"); target != want {
		t.Fatalf("link target = %q, want %q", target, want)
	}
	// Self-update was a no-op: exe untouched.
	if got, _ := os.ReadFile(exe); string(got) != "OLD" {
		t.Fatalf("exe changed to %q, want OLD (no-op self-update)", got)
	}
}

// TestUpdatePullErrorAborts: a pull failure must not be swallowed. Exit 1.
func TestUpdatePullErrorAborts(t *testing.T) {
	r := &run.FakeRunner{}
	rel := release.FakeReleases{Tags: map[string]string{"schuettc/kempt": "v" + version.Number()}}
	_, repo, _ := setupUpdate(t, r, rel)
	r.Responses = map[string]run.Response{
		"git -C " + repo + " pull --rebase --autostash": {Err: fmt.Errorf("merge conflict")},
	}

	var out, errw bytes.Buffer
	code := Dispatch([]string{"update"}, &out, &errw)
	if code != 1 {
		t.Fatalf("exit = %d, want 1; out=%s err=%s", code, out.String(), errw.String())
	}
}

func TestUpdateNoStateIsUsageError(t *testing.T) {
	origLoad := loadState
	loadState = func() (*state.State, bool, error) { return &state.State{}, false, nil }
	t.Cleanup(func() { loadState = origLoad })
	var out, errw bytes.Buffer
	if code := Dispatch([]string{"update"}, &out, &errw); code != 2 {
		t.Fatalf("exit = %d, want 2; err=%s", code, errw.String())
	}
}
