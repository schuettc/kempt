package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/schuettc/kempt/internal/machine"
	"github.com/schuettc/kempt/internal/release"
	"github.com/schuettc/kempt/internal/run"
	"github.com/schuettc/kempt/internal/state"
	tools "github.com/schuettc/tools-common"
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
// Home), and stubs the selfUpdate seam to a no-op so the binary-replace step
// never reaches the network. It returns (home, repo).
func setupUpdate(t *testing.T, r *run.FakeRunner, rel release.Releases) (home, repo string) {
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

	stateStore := &state.Store{Dir: t.TempDir()}
	if err := stateStore.Save(&state.State{RepoDir: repo, Packages: []string{"a"}}); err != nil {
		t.Fatal(err)
	}

	origLoad, origCtx, origSelf := loadState, newContext, selfUpdate
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
	// Default: self-update is a no-op (already latest).
	selfUpdate = func(app *tools.App, out, errw io.Writer) (bool, string, error) {
		return false, "dev", nil
	}
	t.Cleanup(func() {
		loadState, newContext, selfUpdate = origLoad, origCtx, origSelf
	})
	return home, repo
}

// TestUpdateCurrentBinaryConverges: pull succeeds, self-update is a no-op, and
// a files-only manifest converges. Exit 0.
func TestUpdateCurrentBinaryConverges(t *testing.T) {
	r := &run.FakeRunner{}
	rel := release.FakeReleases{}
	home, repo := setupUpdate(t, r, rel)
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
}

// TestUpdatePullErrorAborts: a pull failure must not be swallowed. Exit 1.
func TestUpdatePullErrorAborts(t *testing.T) {
	r := &run.FakeRunner{}
	rel := release.FakeReleases{}
	_, repo := setupUpdate(t, r, rel)
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
