package cli

import (
	"testing"

	"github.com/schuettc/kempt/internal/machine"
	"github.com/schuettc/kempt/internal/manifest"
	"github.com/schuettc/kempt/internal/release"
	"github.com/schuettc/kempt/internal/run"
)

func TestScanTools(t *testing.T) {
	home := t.TempDir()
	fr := &run.FakeRunner{Responses: map[string]run.Response{
		home + "/.local/bin/proj version":    {Stdout: "proj 0.1.1 (a, d)\n"},   // latest tool, behind
		home + "/.local/bin/scratch version": {Stdout: "scratch 0.5.1 (a, d)\n"}, // pinned, matches
	}}
	rel := release.FakeReleases{Files: map[string][]byte{
		"https://tackle.tools/dl/proj/latest": []byte("0.1.2\n"),
	}}
	ctx := &machine.Context{Home: home, OS: "darwin", Arch: "arm64", Runner: fr, Releases: rel, Cache: map[string]string{}}

	pkg := &manifest.Package{Name: "terminal", Steps: []manifest.Step{
		manifest.DownloadStep{Site: "tackle.tools", Tool: "proj", Version: "latest", Bin: "proj"},
		manifest.DownloadStep{Site: "tackle.tools", Tool: "scratch", Version: "0.5.1", Bin: "scratch"},
	}}

	got, err := scanTools(ctx, []*manifest.Package{pkg})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 statuses, got %d", len(got))
	}
	if got[0].Tool != "proj" || got[0].Mode != "latest" || got[0].Installed != "0.1.1" || got[0].Target != "0.1.2" || !got[0].Behind {
		t.Fatalf("proj: %+v", got[0])
	}
	if got[1].Tool != "scratch" || got[1].Mode != "pinned" || got[1].Behind {
		t.Fatalf("scratch should be up-to-date pinned: %+v", got[1])
	}
}
