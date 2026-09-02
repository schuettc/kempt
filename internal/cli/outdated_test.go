package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/schuettc/kempt/internal/machine"
	"github.com/schuettc/kempt/internal/release"
	"github.com/schuettc/kempt/internal/run"
)

// writeTempManifest writes content to kempt.toml in a fresh tempdir and
// returns the dir.
func writeTempManifest(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "kempt.toml")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// stubContext overrides newContext to return a Context whose Home is a fresh
// tempdir (with the binaries named in runResponses created so os.Stat
// passes), scripted with a FakeRunner from runResponses and FakeReleases from
// releaseFiles. It returns a restore func.
func stubContext(t *testing.T, repoDir string, runResponses map[string]string, releaseFiles map[string]string) func() {
	t.Helper()
	home := t.TempDir()
	binDir := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}

	fr := &run.FakeRunner{Responses: map[string]run.Response{}}
	for key, stdout := range runResponses {
		// key is "<bin> version"; the FakeRunner is keyed on the resolved
		// binPath(ctx, bin)+" version".
		bin := strings.TrimSuffix(key, " version")
		fullKey := filepath.Join(binDir, bin) + " version"
		fr.Responses[fullKey] = run.Response{Stdout: stdout}
		// create the binary file so os.Stat passes.
		if err := os.WriteFile(filepath.Join(binDir, bin), []byte(""), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	rel := release.FakeReleases{Files: map[string][]byte{}}
	for url, content := range releaseFiles {
		rel.Files[url] = []byte(content)
	}

	orig := newContext
	newContext = func(dir string) (*machine.Context, error) {
		return &machine.Context{
			Home:     home,
			RepoDir:  dir,
			OS:       "darwin",
			Arch:     "arm64",
			Runner:   fr,
			Releases: rel,
			Cache:    map[string]string{},
		}, nil
	}
	return func() { newContext = orig }
}

func TestOutdatedListsBehind(t *testing.T) {
	dir := writeTempManifest(t, `
[kempt]
spec = 1
[packages.terminal]
  [[packages.terminal.download]]
  site = "tackle.tools"
  tool = "proj"
  version = "latest"
  bin = "proj"
`)
	restore := stubContext(t, dir, map[string]string{
		"proj version": "proj 0.1.1 (a, d)\n",
	}, map[string]string{
		"https://tackle.tools/dl/proj/latest": "0.1.2\n",
	})
	defer restore()

	var out, errw bytes.Buffer
	if err := runOutdated([]string{"-manifest", dir + "/kempt.toml", "-packages", "terminal"}, &out, &errw); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "proj") || !strings.Contains(out.String(), "0.1.1") || !strings.Contains(out.String(), "0.1.2") {
		t.Fatalf("want proj 0.1.1 -> 0.1.2, got: %q", out.String())
	}
}

func TestOutdatedUpToDate(t *testing.T) {
	dir := writeTempManifest(t, `
[kempt]
spec = 1
[packages.terminal]
  [[packages.terminal.download]]
  site = "tackle.tools"
  tool = "proj"
  version = "0.1.2"
  bin = "proj"
`)
	restore := stubContext(t, dir, map[string]string{
		"proj version": "proj 0.1.2 (a, d)\n",
	}, map[string]string{})
	defer restore()

	var out, errw bytes.Buffer
	if err := runOutdated([]string{"-manifest", dir + "/kempt.toml", "-packages", "terminal"}, &out, &errw); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "everything up to date") {
		t.Fatalf("want up to date message, got: %q", out.String())
	}
}
