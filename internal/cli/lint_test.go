package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "kempt.toml")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLintClean(t *testing.T) {
	p := writeTemp(t, "[kempt]\nspec = 1\n[packages.a]\ndescription = \"a\"\n")
	var out, errw bytes.Buffer
	if got := Dispatch([]string{"lint", p}, &out, &errw); got != 0 {
		t.Fatalf("exit = %d, stderr = %s", got, errw.String())
	}
}

func TestLintFindings(t *testing.T) {
	p := writeTemp(t, "[kempt]\nspec = 1\nbogus = 1\n")
	var out, errw bytes.Buffer
	if got := Dispatch([]string{"lint", p}, &out, &errw); got != 1 {
		t.Fatalf("exit = %d", got)
	}
	if !strings.Contains(out.String(), "unknown key") {
		t.Fatalf("stdout = %q", out.String())
	}
}

func TestLintMissingFile(t *testing.T) {
	var out, errw bytes.Buffer
	if got := Dispatch([]string{"lint", "/nonexistent/kempt.toml"}, &out, &errw); got != 2 {
		t.Fatalf("exit = %d", got)
	}
}
