package handlers

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/schuettc/kempt/internal/engine"
	"github.com/schuettc/kempt/internal/machine"
	"github.com/schuettc/kempt/internal/manifest"
	"github.com/schuettc/kempt/internal/run"
)

func serviceHandler(t *testing.T) engine.Handler {
	t.Helper()
	h, ok := engine.HandlerFor("service")
	if !ok {
		t.Fatal("service handler not registered")
	}
	return h
}

func svcCtx(t *testing.T, r run.Runner, uid int) *machine.Context {
	t.Helper()
	tmp := t.TempDir()
	return &machine.Context{
		Home:    tmp,
		RepoDir: tmp,
		OS:      "darwin",
		Arch:    "arm64",
		UID:     uid,
		Runner:  r,
	}
}

var goldenStep = manifest.ServiceStep{
	Label:   "com.example.svc",
	Program: []string{"/usr/bin/foo", "--flag=a&b"},
}

// goldenPlist is the exact expected render for goldenStep. Built with explicit
// \t / \n so the tab layout is unambiguous.
const goldenPlist = "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n" +
	"<!DOCTYPE plist PUBLIC \"-//Apple//DTD PLIST 1.0//EN\" \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\">\n" +
	"<plist version=\"1.0\">\n" +
	"<dict>\n" +
	"\t<key>KeepAlive</key>\n" +
	"\t<true/>\n" +
	"\t<key>Label</key>\n" +
	"\t<string>com.example.svc</string>\n" +
	"\t<key>ProgramArguments</key>\n" +
	"\t<array>\n" +
	"\t\t<string>/usr/bin/foo</string>\n" +
	"\t\t<string>--flag=a&amp;b</string>\n" +
	"\t</array>\n" +
	"\t<key>RunAtLoad</key>\n" +
	"\t<true/>\n" +
	"</dict>\n" +
	"</plist>\n"

func TestServiceRenderGolden(t *testing.T) {
	got := renderPlist(goldenStep)
	if got != goldenPlist {
		t.Fatalf("render mismatch\n--- got ---\n%q\n--- want ---\n%q", got, goldenPlist)
	}
}

func plistPath(ctx *machine.Context, label string) string {
	return filepath.Join(ctx.Home, "Library", "LaunchAgents", label+".plist")
}

func TestServiceInspectNonDarwinBlocked(t *testing.T) {
	h := serviceHandler(t)
	ctx := svcCtx(t, &run.FakeRunner{}, 501)
	ctx.OS = "linux"
	d, err := h.Inspect(ctx, goldenStep)
	if err != nil {
		t.Fatal(err)
	}
	if d.Op != engine.OpBlocked {
		t.Fatalf("op = %v (%q), want blocked", d.Op, d.Detail)
	}
}

func TestServiceInspectMissingChange(t *testing.T) {
	h := serviceHandler(t)
	ctx := svcCtx(t, &run.FakeRunner{}, 501)
	d, err := h.Inspect(ctx, goldenStep)
	if err != nil {
		t.Fatal(err)
	}
	if d.Op != engine.OpChange {
		t.Fatalf("op = %v (%q), want change", d.Op, d.Detail)
	}
}

func TestServiceInspectDiffersChange(t *testing.T) {
	h := serviceHandler(t)
	ctx := svcCtx(t, &run.FakeRunner{}, 501)
	p := plistPath(ctx, goldenStep.Label)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := h.Inspect(ctx, goldenStep)
	if err != nil {
		t.Fatal(err)
	}
	if d.Op != engine.OpChange {
		t.Fatalf("op = %v (%q), want change", d.Op, d.Detail)
	}
}

func TestServiceInspectSameNoop(t *testing.T) {
	h := serviceHandler(t)
	ctx := svcCtx(t, &run.FakeRunner{}, 501)
	p := plistPath(ctx, goldenStep.Label)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(goldenPlist), 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := h.Inspect(ctx, goldenStep)
	if err != nil {
		t.Fatal(err)
	}
	if d.Op != engine.OpNoop {
		t.Fatalf("op = %v (%q), want noop", d.Op, d.Detail)
	}
}

func TestServiceApplyWritesAndReloads(t *testing.T) {
	h := serviceHandler(t)
	fr := &run.FakeRunner{Responses: map[string]run.Response{}}
	ctx := svcCtx(t, fr, 501)
	p := plistPath(ctx, goldenStep.Label)
	bootout := "launchctl bootout gui/501 " + p
	bootstrap := "launchctl bootstrap gui/501 " + p
	fr.Responses[bootstrap] = run.Response{}
	// bootout intentionally left unscripted: its error must be ignored.

	if err := h.Apply(ctx, goldenStep); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("plist not written: %v", err)
	}
	if string(b) != goldenPlist {
		t.Fatalf("plist content mismatch\n--- got ---\n%q", string(b))
	}
	want := []string{bootout, bootstrap}
	if len(fr.Calls) != 2 || fr.Calls[0] != want[0] || fr.Calls[1] != want[1] {
		t.Fatalf("calls = %v, want %v", fr.Calls, want)
	}
}

func TestServiceApplyNoopIssuesNoCommands(t *testing.T) {
	h := serviceHandler(t)
	fr := &run.FakeRunner{Responses: map[string]run.Response{}}
	ctx := svcCtx(t, fr, 501)
	p := plistPath(ctx, goldenStep.Label)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(goldenPlist), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := h.Apply(ctx, goldenStep); err != nil {
		t.Fatal(err)
	}
	if len(fr.Calls) != 0 {
		t.Fatalf("calls = %v, want none", fr.Calls)
	}
}
