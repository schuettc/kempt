package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/schuettc/kempt/internal/run"
	"github.com/schuettc/kempt/internal/state"
)

// TestPlanSavedSelectionExitZero verifies the post-init front-door flow:
// after `kempt init --profile developer` the state has both Profile and Packages
// set. A subsequent plain `kempt plan -manifest ...` must exit 0, not 2.
// The bug was that resolveSelection returned (st.Profile, st.Packages) causing
// engine.Select to reject "both profile and packages".
func TestPlanSavedSelectionExitZero(t *testing.T) {
	// Build a minimal manifest with package "mypkg" in a tempdir.
	dir := t.TempDir()
	manifestContent := `
[kempt]
spec = 1

[packages.mypkg]
description = "my package"
  [[packages.mypkg.symlink]]
  from = "src/rc"
  to = "~/.rc"
`
	manifestPath := filepath.Join(dir, "kempt.toml")
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0o644); err != nil {
		t.Fatal(err)
	}
	// Create the symlink source so plan can inspect it.
	if err := os.MkdirAll(filepath.Join(dir, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "src", "rc"), []byte("rc\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Override loadState to return a state that has BOTH Profile and Packages
	// set (as init would produce), plus the repo dir pointing at our tempdir.
	origLoad := loadState
	loadState = func() (*state.State, bool, error) {
		return &state.State{
			RepoDir:  dir,
			Profile:  "developer",
			Packages: []string{"mypkg"},
		}, true, nil
	}
	t.Cleanup(func() { loadState = origLoad })

	// Override newContext to use a hermetic home so symlink inspection is safe.
	home := t.TempDir()
	withContextHome(t, home, &run.FakeRunner{})

	var out, errw bytes.Buffer
	code := Dispatch([]string{"plan", "-manifest", manifestPath}, &out, &errw)
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (saved-selection should not trigger both-set error)\nstdout: %s\nstderr: %s",
			code, out.String(), errw.String())
	}
}
