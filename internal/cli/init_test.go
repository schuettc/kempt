package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/schuettc/kempt/internal/machine"
	"github.com/schuettc/kempt/internal/picker"
	"github.com/schuettc/kempt/internal/run"
	"github.com/schuettc/kempt/internal/state"
)

const initManifest = `
[kempt]
spec = 1

[packages.tools]
description = "developer tools"
  [[packages.tools.symlink]]
  from = "src/rc"
  to = "~/.rc"

[profiles.developer]
description = "developer profile"
packages = ["tools"]
`

const initTestURL = "https://example.com/dotfiles.git"

// setupInit writes the manifest+source into a repo tempdir, points saveState at
// a tempdir store, installs a newContext returning a Context with the given
// FakeRunner and a tempdir Home, and scripts the git seams. It returns
// (stateStore, home, dir). By default RemoteURL errors (dir is "not a repo")
// and `git clone <url> <dir>` succeeds.
func setupInit(t *testing.T, r *run.FakeRunner) (store *state.Store, home, dir string) {
	t.Helper()
	dir = t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "kempt.toml"), []byte(initManifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "src", "rc"), []byte("rc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	home = t.TempDir()
	store = &state.Store{Dir: t.TempDir()}

	origSave, origCtx, origPicker := saveState, newContext, pickerRun
	saveState = func(s *state.State) error { return store.Save(s) }
	newContext = func(repoDir string) (*machine.Context, error) {
		return &machine.Context{
			Home:    home,
			RepoDir: repoDir,
			OS:      "darwin",
			Arch:    "arm64",
			Runner:  r,
			Cache:   map[string]string{},
		}, nil
	}
	t.Cleanup(func() {
		saveState, newContext, pickerRun = origSave, origCtx, origPicker
	})
	_ = origPicker
	return store, home, dir
}

func TestInitProfileNonInteractive(t *testing.T) {
	r := &run.FakeRunner{}
	store, home, dir := setupInit(t, r)
	r.Responses = map[string]run.Response{
		"git -C " + dir + " remote get-url origin": {Err: os.ErrNotExist},
		"git clone " + initTestURL + " " + dir:     {Stdout: ""},
	}

	var out, errw bytes.Buffer
	code := Dispatch([]string{"init", initTestURL, "-dir", dir, "-profile", "developer", "-yes"}, &out, &errw)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; out=%s err=%s", code, out.String(), errw.String())
	}

	// Clone was called.
	cloned := false
	for _, c := range r.Calls {
		if c == "git clone "+initTestURL+" "+dir {
			cloned = true
		}
	}
	if !cloned {
		t.Fatalf("git clone not called; calls=%v", r.Calls)
	}

	// State saved.
	st, existed, err := store.Load()
	if err != nil || !existed {
		t.Fatalf("state not saved: existed=%v err=%v", existed, err)
	}
	if st.RepoDir != dir || st.RepoURL != initTestURL || st.Profile != "developer" {
		t.Fatalf("state = %+v, want dir/url/profile set", st)
	}
	if len(st.Packages) != 1 || st.Packages[0] != "tools" {
		t.Fatalf("state packages = %v, want [tools]", st.Packages)
	}
	if st.AutoApplyFiles {
		t.Fatalf("AutoApplyFiles = true, want false")
	}

	// Applied: symlink created.
	link := filepath.Join(home, ".rc")
	if target, err := os.Readlink(link); err != nil {
		t.Fatalf("symlink not created: %v", err)
	} else if want := filepath.Join(dir, "src", "rc"); target != want {
		t.Fatalf("link target = %q, want %q", target, want)
	}
}

func TestInitInteractivePicker(t *testing.T) {
	r := &run.FakeRunner{}
	store, home, dir := setupInit(t, r)
	r.Responses = map[string]run.Response{
		"git -C " + dir + " remote get-url origin": {Err: os.ErrNotExist},
		"git clone " + initTestURL + " " + dir:     {Stdout: ""},
	}
	pickerRun = func(profiles []picker.Profile, items []picker.Item) (picker.Result, error) {
		return picker.Result{Profile: "developer", Packages: []string{"tools"}, Confirmed: true}, nil
	}
	setStdin(t, "y\n")

	var out, errw bytes.Buffer
	code := Dispatch([]string{"init", initTestURL, "-dir", dir}, &out, &errw)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; out=%s err=%s", code, out.String(), errw.String())
	}
	st, existed, err := store.Load()
	if err != nil || !existed {
		t.Fatalf("state not saved: existed=%v err=%v", existed, err)
	}
	if st.Profile != "developer" || len(st.Packages) != 1 || st.Packages[0] != "tools" {
		t.Fatalf("state = %+v, want picker selection", st)
	}
	if _, err := os.Readlink(filepath.Join(home, ".rc")); err != nil {
		t.Fatalf("symlink not created: %v", err)
	}
}

func TestInitCancelledPicker(t *testing.T) {
	r := &run.FakeRunner{}
	store, _, dir := setupInit(t, r)
	r.Responses = map[string]run.Response{
		"git -C " + dir + " remote get-url origin": {Err: os.ErrNotExist},
		"git clone " + initTestURL + " " + dir:     {Stdout: ""},
	}
	pickerRun = func(profiles []picker.Profile, items []picker.Item) (picker.Result, error) {
		return picker.Result{Confirmed: false}, nil
	}
	var out, errw bytes.Buffer
	code := Dispatch([]string{"init", initTestURL, "-dir", dir}, &out, &errw)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; out=%s err=%s", code, out.String(), errw.String())
	}
	if _, existed, _ := store.Load(); existed {
		t.Fatalf("state should not be saved on cancel")
	}
}

func TestInitUnknownProfile(t *testing.T) {
	r := &run.FakeRunner{}
	_, _, dir := setupInit(t, r)
	r.Responses = map[string]run.Response{
		"git -C " + dir + " remote get-url origin": {Err: os.ErrNotExist},
		"git clone " + initTestURL + " " + dir:     {Stdout: ""},
	}
	var out, errw bytes.Buffer
	code := Dispatch([]string{"init", initTestURL, "-dir", dir, "-profile", "nope", "-yes"}, &out, &errw)
	if code != 2 {
		t.Fatalf("exit = %d, want 2; err=%s", code, errw.String())
	}
}

func TestInitYesWithoutProfile(t *testing.T) {
	r := &run.FakeRunner{}
	_, _, dir := setupInit(t, r)
	r.Responses = map[string]run.Response{
		"git -C " + dir + " remote get-url origin": {Err: os.ErrNotExist},
		"git clone " + initTestURL + " " + dir:     {Stdout: ""},
	}
	var out, errw bytes.Buffer
	code := Dispatch([]string{"init", initTestURL, "-dir", dir, "-yes"}, &out, &errw)
	if code != 2 {
		t.Fatalf("exit = %d, want 2; err=%s", code, errw.String())
	}
}

func TestInitNoURLEmptyDir(t *testing.T) {
	r := &run.FakeRunner{}
	_, _, dir := setupInit(t, r)
	// dir is not a repo: RemoteURL errors, and no url was provided.
	r.Responses = map[string]run.Response{
		"git -C " + dir + " remote get-url origin": {Err: os.ErrNotExist},
	}
	var out, errw bytes.Buffer
	code := Dispatch([]string{"init", "-dir", dir, "-profile", "developer", "-yes"}, &out, &errw)
	if code != 2 {
		t.Fatalf("exit = %d, want 2; err=%s", code, errw.String())
	}
}
