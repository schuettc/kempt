package run

import (
	"errors"
	"os/exec"
	"testing"
)

func TestFakeRunnerScripted(t *testing.T) {
	scriptedErr := errors.New("boom")
	f := &FakeRunner{
		Responses: map[string]Response{
			"git":               {Stdout: "git-ok\n"},
			"git status":        {Stdout: "clean\n"},
			"git log --oneline": {Stdout: "abc123 msg\n"},
			"git fail":          {Err: scriptedErr},
			"lookpath git":      {Stdout: "/usr/bin/git"},
		},
	}

	// Zero-arg call uses the bare name as key.
	out, err := f.Run("git")
	if err != nil {
		t.Fatalf("Run zero-arg: unexpected error: %v", err)
	}
	if out != "git-ok\n" {
		t.Errorf("Run zero-arg stdout = %q; want %q", out, "git-ok\n")
	}
	if len(f.Calls) != 1 || f.Calls[0] != "git" {
		t.Errorf("Run zero-arg Calls = %v; want [\"git\"]", f.Calls)
	}

	// Scripted key returns correct stdout.
	out, err = f.Run("git", "status")
	if err != nil {
		t.Fatalf("Run scripted: unexpected error: %v", err)
	}
	if out != "clean\n" {
		t.Errorf("Run scripted stdout = %q; want %q", out, "clean\n")
	}

	// Another scripted key.
	out, err = f.Run("git", "log", "--oneline")
	if err != nil {
		t.Fatalf("Run scripted 2: unexpected error: %v", err)
	}
	if out != "abc123 msg\n" {
		t.Errorf("Run scripted 2 stdout = %q; want %q", out, "abc123 msg\n")
	}

	// Scripted error is returned.
	_, err = f.Run("git", "fail")
	if !errors.Is(err, scriptedErr) {
		t.Errorf("Run scripted error = %v; want %v", err, scriptedErr)
	}

	// Unscripted key returns an error naming the key.
	_, err = f.Run("unknown", "cmd")
	if err == nil {
		t.Fatal("Run unscripted: expected error, got nil")
	}
	const wantMsg = "unscripted command: unknown cmd"
	if err.Error() != wantMsg {
		t.Errorf("Run unscripted error = %q; want %q", err.Error(), wantMsg)
	}

	// Calls records all invocations in order (including the zero-arg call).
	wantCalls := []string{"git", "git status", "git log --oneline", "git fail", "unknown cmd"}
	if len(f.Calls) != len(wantCalls) {
		t.Fatalf("Calls length = %d; want %d; got %v", len(f.Calls), len(wantCalls), f.Calls)
	}
	for i, want := range wantCalls {
		if f.Calls[i] != want {
			t.Errorf("Calls[%d] = %q; want %q", i, f.Calls[i], want)
		}
	}

	// LookPath scripted.
	path, err := f.LookPath("git")
	if err != nil {
		t.Fatalf("LookPath scripted: unexpected error: %v", err)
	}
	if path != "/usr/bin/git" {
		t.Errorf("LookPath scripted = %q; want %q", path, "/usr/bin/git")
	}

	// LookPath unscripted returns ErrNotFound.
	_, err = f.LookPath("notfound")
	if !errors.Is(err, exec.ErrNotFound) {
		t.Errorf("LookPath unscripted error = %v; want exec.ErrNotFound", err)
	}

	// Calls now includes lookpath entries.
	wantLookpathCalls := []string{"lookpath git", "lookpath notfound"}
	gotLookpathCalls := f.Calls[5:]
	if len(gotLookpathCalls) != len(wantLookpathCalls) {
		t.Fatalf("LookPath Calls length = %d; want %d; got %v", len(gotLookpathCalls), len(wantLookpathCalls), gotLookpathCalls)
	}
	for i, want := range wantLookpathCalls {
		if gotLookpathCalls[i] != want {
			t.Errorf("LookPath Calls[%d] = %q; want %q", i, gotLookpathCalls[i], want)
		}
	}
}
