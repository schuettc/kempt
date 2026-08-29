package cli

import (
	"bytes"
	"strings"
	"testing"
)

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
