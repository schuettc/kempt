package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/schuettc/kempt/internal/state"
)

// TestMain stubs loadState to an empty absent state so that plan/apply/verify
// tests are hermetic: they never read ~/.local/share/kempt/state.json from the
// developer's machine. Tests that need specific saved state (adopt/drop tests)
// override loadState/saveState locally via withAdoptEnv.
func TestMain(m *testing.M) {
	loadState = func() (*state.State, bool, error) { return &state.State{}, false, nil }
	os.Exit(m.Run())
}

func TestDispatchNoArgsIsUsageError(t *testing.T) {
	var out, errw bytes.Buffer
	if got := Dispatch(nil, &out, &errw); got != 2 {
		t.Fatalf("exit = %d, want 2", got)
	}
	if !strings.Contains(errw.String(), "usage: kempt") {
		t.Fatalf("stderr missing usage block: %q", errw.String())
	}
}

func TestDispatchVersion(t *testing.T) {
	var out, errw bytes.Buffer
	if got := Dispatch([]string{"version"}, &out, &errw); got != 0 {
		t.Fatalf("exit = %d, want 0", got)
	}
	if !strings.Contains(out.String(), "kempt ") {
		t.Fatalf("stdout = %q, want version line", out.String())
	}
}

func TestDispatchUnknownCommand(t *testing.T) {
	var out, errw bytes.Buffer
	if got := Dispatch([]string{"frobnicate"}, &out, &errw); got != 2 {
		t.Fatalf("exit = %d, want 2", got)
	}
}
