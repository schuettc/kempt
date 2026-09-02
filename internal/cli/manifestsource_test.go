package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadManifestSourceLocalPath(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "kempt.toml")
	if err := os.WriteFile(p, []byte("[kempt]\nspec = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	src, repoDir, name, err := loadManifestSource(p, strings.NewReader(""))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), "spec = 1") {
		t.Fatalf("src = %q", src)
	}
	if repoDir != dir {
		t.Fatalf("repoDir = %q, want %q", repoDir, dir)
	}
	if name != p {
		t.Fatalf("name = %q, want %q", name, p)
	}
}

func TestLoadManifestSourceStdin(t *testing.T) {
	src, repoDir, name, err := loadManifestSource("-", strings.NewReader("[kempt]\nspec = 1\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), "spec = 1") {
		t.Fatalf("src = %q", src)
	}
	cwd, _ := os.Getwd()
	if repoDir != cwd {
		t.Fatalf("repoDir = %q, want cwd %q", repoDir, cwd)
	}
	if name != "<stdin>" {
		t.Fatalf("name = %q, want <stdin>", name)
	}
}

func TestLoadManifestSourceURL(t *testing.T) {
	orig := fetchManifestURL
	t.Cleanup(func() { fetchManifestURL = orig })
	fetchManifestURL = func(url string) ([]byte, error) {
		if url != "https://example.test/kempt.toml" {
			t.Fatalf("unexpected url %q", url)
		}
		return []byte("[kempt]\nspec = 1\n"), nil
	}
	src, repoDir, name, err := loadManifestSource("https://example.test/kempt.toml", strings.NewReader(""))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), "spec = 1") {
		t.Fatalf("src = %q", src)
	}
	cwd, _ := os.Getwd()
	if repoDir != cwd {
		t.Fatalf("repoDir = %q, want cwd %q", repoDir, cwd)
	}
	if name != "https://example.test/kempt.toml" {
		t.Fatalf("name = %q", name)
	}
}

func TestApplyStdinRequiresYes(t *testing.T) {
	var out, errw strings.Builder
	err := runApply([]string{"-manifest", "-"}, &out, &errw)
	if err == nil || !strings.Contains(err.Error(), "requires -yes") {
		t.Fatalf("err = %v, want a 'requires -yes' UsageError", err)
	}
}
