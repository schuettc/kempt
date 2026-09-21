package doctor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/schuettc/kempt/internal/manifest"
)

func TestD4ForeignFile(t *testing.T) {
	ctx := ctxT(t)
	to := filepath.Join(t.TempDir(), "link")
	writeFile(t, to, "real file where a symlink is expected")
	pkgs := []*manifest.Package{{Name: "p", Steps: []manifest.Step{
		manifest.SymlinkStep{From: "src", To: to},
	}}}
	got := CheckSymlinks(ctx, pkgs)
	if len(got) != 1 || got[0].Severity != Warn {
		t.Fatalf("want 1 warn (foreign), got %+v", got)
	}
}

func TestD4BrokenLink(t *testing.T) {
	ctx := ctxT(t)
	to := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(filepath.Join(t.TempDir(), "does-not-exist"), to); err != nil {
		t.Fatal(err)
	}
	got := CheckSymlinks(ctx, []*manifest.Package{{Name: "p", Steps: []manifest.Step{
		manifest.SymlinkStep{From: "src", To: to},
	}}})
	if len(got) != 1 || got[0].Severity != Error {
		t.Fatalf("want 1 error (broken), got %+v", got)
	}
}

func TestD4AbsentNoFinding(t *testing.T) {
	ctx := ctxT(t)
	to := filepath.Join(t.TempDir(), "nope")
	if got := CheckSymlinks(ctx, []*manifest.Package{{Name: "p", Steps: []manifest.Step{
		manifest.SymlinkStep{From: "src", To: to},
	}}}); len(got) != 0 {
		t.Fatalf("absent target is plan's job, want none, got %+v", got)
	}
}
