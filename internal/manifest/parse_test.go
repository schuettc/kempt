// internal/manifest/parse_test.go
package manifest

import (
	"fmt"
	"os"
	"testing"

	"github.com/BurntSushi/toml"
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

func TestStepsKeepWrittenOrderAcrossKinds(t *testing.T) {
	src := []byte(`
[kempt]
spec = 1
[packages.x]
description = "order test"
[[packages.x.symlink]]
from = "a"
to = "~/a"
[[packages.x.install]]
brew = { formulas = ["jq"] }
[[packages.x.symlink]]
from = "b"
to = "~/b"
`)
	m, findings := Parse(src)
	if len(findings) != 0 {
		t.Fatalf("findings: %v", findings)
	}
	got := []string{}
	for _, s := range m.Packages["x"].Steps {
		got = append(got, s.Kind())
	}
	want := []string{"symlink", "install", "symlink"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
}

func TestIsFreeform(t *testing.T) {
	cases := []struct {
		name string
		key  toml.Key
		want bool
	}{
		{
			name: "suppressed merge sub-key",
			key:  toml.Key{"packages", "muster", "json-merge", "merge", "mcpServers"},
			want: true,
		},
		{
			name: "top-level json-merge.merge without packages prefix",
			key:  toml.Key{"json-merge", "merge"},
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isFreeform(tc.key); got != tc.want {
				t.Fatalf("isFreeform(%v) = %v, want %v", []string(tc.key), got, tc.want)
			}
		})
	}
}

func TestParseUnknownKeyIsFinding(t *testing.T) {
	src := []byte("[kempt]\nspec = 1\nbogus = true\n")
	_, findings := Parse(src)
	if len(findings) == 0 {
		t.Fatal("want finding for unknown key 'bogus'")
	}
}
