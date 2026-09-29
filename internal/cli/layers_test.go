package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/schuettc/kempt/internal/layers"
	"github.com/schuettc/kempt/internal/machine"
	"github.com/schuettc/kempt/internal/release"
	"github.com/schuettc/kempt/internal/run"
	"github.com/schuettc/kempt/internal/state"
	tools "github.com/schuettc/tools-common"
)

// The base owns settings.json packages with arrays = "replace".
const layeredBase = `
[kempt]
spec = 1
[packages.pi]
  [[packages.pi.json-merge]]
  file = "~/settings.json"
  arrays = "replace"
  merge = { packages = ["npm:a"] }
`

const workLayer = `
[kempt]
spec = 1
layer = "user"
[packages.mkt]
  [[packages.mkt.json-merge]]
  file = "~/settings.json"
  arrays = "replace"
  merge = { packages = ["git:mkt"] }
`

// layeredMachine saves a state with the base repo and one work layer, and
// points the CLI's state and context seams at temp dirs.
func layeredMachine(t *testing.T, layerSrc string, extra ...state.Layer) (home, repo, layerDir string, store *state.Store, fr *run.FakeRunner) {
	t.Helper()
	repo, home, layerDir = t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "kempt.toml"), []byte(layeredBase), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(layerDir, "kempt.toml"), []byte(layerSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	store = &state.Store{Dir: t.TempDir()}
	st := &state.State{RepoDir: repo, Packages: []string{"pi"}, Layers: append([]state.Layer{
		{Name: "work", Scope: "user", Source: state.LayerSource{Kind: "path", Dir: layerDir}, Packages: []string{"mkt"}},
	}, extra...)}
	if err := store.Save(st); err != nil {
		t.Fatal(err)
	}
	fr = &run.FakeRunner{Responses: map[string]run.Response{
		"git -C " + repo + " pull --rebase --autostash": {},
	}}
	origLoad, origSave, origCtx, origSelf := loadState, saveState, newContext, selfUpdate
	loadState = func() (*state.State, bool, error) { return store.Load() }
	saveState = func(s *state.State) error { return store.Save(s) }
	newContext = func(dir string) (*machine.Context, error) {
		return &machine.Context{Home: home, RepoDir: dir, OS: "darwin", Arch: "arm64", Runner: fr,
			Releases: release.FakeReleases{}, Cache: map[string]string{}}, nil
	}
	selfUpdate = func(*tools.App, io.Writer, io.Writer) (bool, string, error) { return false, "dev", nil }
	t.Cleanup(func() { loadState, saveState, newContext, selfUpdate = origLoad, origSave, origCtx, origSelf })
	return
}

func settingsPackages(t *testing.T, home string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(home, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var v struct{ Packages []string }
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	return strings.Join(v.Packages, ",")
}

// The regression layers fix: a layer's entry in a replace-owned array survives
// every converge, apply and update alike.
func TestLayerAdditionSurvivesApplyAndUpdate(t *testing.T) {
	home, _, _, _, _ := layeredMachine(t, workLayer)
	var out, errw bytes.Buffer
	if code := Dispatch([]string{"apply", "-yes"}, &out, &errw); code != 0 {
		t.Fatalf("apply exit %d: %s %s", code, out.String(), errw.String())
	}
	if got := settingsPackages(t, home); got != "npm:a,git:mkt" {
		t.Fatalf("after apply: %s", got)
	}
	for i := 0; i < 2; i++ {
		out.Reset()
		if code := Dispatch([]string{"update"}, &out, &errw); code != 0 {
			t.Fatalf("update exit %d: %s %s", code, out.String(), errw.String())
		}
		if got := settingsPackages(t, home); got != "npm:a,git:mkt" {
			t.Fatalf("after update %d: %s\n%s", i+1, got, out.String())
		}
	}
}

// An explicit -manifest (or -packages) is a one-off run on that manifest only.
func TestExplicitSelectionIgnoresLayers(t *testing.T) {
	home, repo, _, _, _ := layeredMachine(t, workLayer)
	var out, errw bytes.Buffer
	if code := Dispatch([]string{"apply", "-yes", "-manifest", filepath.Join(repo, "kempt.toml"), "-packages", "pi"}, &out, &errw); code != 0 {
		t.Fatalf("exit %d: %s %s", code, out.String(), errw.String())
	}
	if got := settingsPackages(t, home); got != "npm:a" {
		t.Errorf("layers applied to an explicit selection: %s", got)
	}
}

func TestPlanNamesLayerPackagesAndReportsSkippedAndHeld(t *testing.T) {
	proj := t.TempDir()
	if err := os.MkdirAll(filepath.Join(proj, ".kempt"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proj, ".kempt", "kempt.toml"), []byte("[kempt]\nspec = 1\nlayer = \"project\"\n[packages.dev]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	layeredMachine(t, workLayer,
		state.Layer{Name: "gone", Scope: "user", Source: state.LayerSource{Kind: "path", Dir: "/nonexistent/kempt-layer"}},
		state.Layer{Name: "galley", Scope: "project", Project: proj, Approved: "sha256:stale",
			Source: state.LayerSource{Kind: "path", Dir: proj, File: ".kempt/kempt.toml"}},
	)
	var out, errw bytes.Buffer
	if code := Dispatch([]string{"plan"}, &out, &errw); code != 0 {
		t.Fatalf("exit %d: %s %s", code, out.String(), errw.String())
	}
	for _, want := range []string{
		"package pi\n",
		"(from base, work)",
		"skipped layer gone: /nonexistent/kempt-layer/kempt.toml not found\n",
		"held galley (layer changed since you applied it): run kempt layer apply galley\n",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("plan output missing %q:\n%s", want, out.String())
		}
	}
}

func TestLayerFindingsFailTheRun(t *testing.T) {
	layeredMachine(t, "[kempt]\nspec = 1\n[packages.mkt]\n")
	var out, errw bytes.Buffer
	if code := Dispatch([]string{"plan"}, &out, &errw); code == 0 {
		t.Fatalf("a layer with findings did not fail the plan: %s", out.String())
	}
	if !strings.Contains(errw.String(), "layer work") || !strings.Contains(errw.String(), "kempt.layer") {
		t.Errorf("findings not reported: %s", errw.String())
	}
}

var _ = layers.DefaultFile
