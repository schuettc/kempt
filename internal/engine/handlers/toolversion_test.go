package handlers

import (
	"testing"

	"github.com/schuettc/kempt/internal/machine"
	"github.com/schuettc/kempt/internal/release"
	"github.com/schuettc/kempt/internal/run"
)

func ctxWith(r run.Runner, rel release.Releases) *machine.Context {
	return &machine.Context{Home: "/home/x", OS: "darwin", Arch: "arm64", Runner: r, Releases: rel, Cache: map[string]string{}}
}

func TestInstalledToolVersion(t *testing.T) {
	fr := &run.FakeRunner{Responses: map[string]run.Response{
		"/home/x/.local/bin/proj version":  {Stdout: "proj 0.1.2 (9f3a22a, 2026-09-02)\n"},
		"/home/x/.local/bin/dev version":   {Stdout: "dev dev\n"},
		"/home/x/.local/bin/gone version":  {Err: run.ErrForTest},
	}}
	ctx := ctxWith(fr, nil)

	if v, ok := InstalledToolVersion(ctx, "proj"); !ok || v != "0.1.2" {
		t.Fatalf("proj: got (%q,%v), want (0.1.2,true)", v, ok)
	}
	if _, ok := InstalledToolVersion(ctx, "dev"); ok {
		t.Fatalf("dev build should be unknown")
	}
	if _, ok := InstalledToolVersion(ctx, "gone"); ok {
		t.Fatalf("missing binary should be unknown")
	}
}

func TestIsPinnedVersion(t *testing.T) {
	for v, want := range map[string]bool{"0.1.2": true, "1.0.0-rc.1": true, "": false, "latest": false} {
		if got := IsPinnedVersion(v); got != want {
			t.Fatalf("IsPinnedVersion(%q)=%v want %v", v, got, want)
		}
	}
}

func TestLatestVersion(t *testing.T) {
	rel := release.FakeReleases{Files: map[string][]byte{
		"https://tackle.tools/dl/proj/latest": []byte("0.1.2\n"),
	}}
	ctx := ctxWith(nil, rel)
	v, err := LatestVersion(ctx, "tackle.tools", "proj")
	if err != nil || v != "0.1.2" {
		t.Fatalf("got (%q,%v), want (0.1.2,nil)", v, err)
	}
}

func TestSemverNewer(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"0.1.2", "0.1.1", true},
		{"0.1.1", "0.1.2", false},
		{"0.1.1", "0.1.1", false},
		{"0.7.0", "0.7.0-schuettc.2", true},
		{"0.7.0-schuettc.2", "0.7.0", false},
		{"not-a-version", "0.1.1", false},
		{"0.1.1", "not-a-version", false},
	}
	for _, c := range cases {
		if got := SemverNewer(c.a, c.b); got != c.want {
			t.Errorf("SemverNewer(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}
