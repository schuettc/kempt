// internal/manifest/parse_test.go
package manifest

import (
	"os"
	"testing"
)

func mustParse(t *testing.T, path string) *Manifest {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	m, findings := Parse(src)
	if len(findings) != 0 {
		t.Fatalf("unexpected findings: %v", findings)
	}
	return m
}

func TestParseReference(t *testing.T) {
	m := mustParse(t, "testdata/reference.toml")
	if m.Spec != 1 {
		t.Fatalf("spec = %d, want 1", m.Spec)
	}
	core, ok := m.Packages["core"]
	if !ok {
		t.Fatal("missing package core")
	}
	if len(core.Steps) != 2 {
		t.Fatalf("core steps = %d, want 2", len(core.Steps))
	}
	if core.Steps[0].Kind() != "install" || core.Steps[1].Kind() != "symlink" {
		t.Fatalf("step order = %s,%s", core.Steps[0].Kind(), core.Steps[1].Kind())
	}
	if core.Steps[0].Class() != ClassSoftware || core.Steps[1].Class() != ClassFiles {
		t.Fatal("safety classes wrong")
	}
	dev := m.Profiles["developer"]
	if dev == nil || len(dev.Packages) == 0 {
		t.Fatal("developer profile missing")
	}
}

func TestParseUnknownKeyIsFinding(t *testing.T) {
	src := []byte("[kempt]\nspec = 1\nbogus = true\n")
	_, findings := Parse(src)
	if len(findings) == 0 {
		t.Fatal("want finding for unknown key 'bogus'")
	}
}
