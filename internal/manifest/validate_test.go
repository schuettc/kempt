package manifest

import (
	"strings"
	"testing"
)

func findingsFor(t *testing.T, src string) []Finding {
	t.Helper()
	m, pf := Parse([]byte(src))
	if len(pf) != 0 {
		t.Fatalf("parse findings: %v", pf)
	}
	return Validate(m)
}

func TestValidateCycle(t *testing.T) {
	f := findingsFor(t, `
[kempt]
spec = 1
[packages.a]
description = "a"
needs = ["b"]
[packages.b]
description = "b"
needs = ["a"]
`)
	if len(f) == 0 || !strings.Contains(f[0].Msg, "cycle") {
		t.Fatalf("want cycle finding, got %v", f)
	}
}

func TestValidateUnknownNeed(t *testing.T) {
	f := findingsFor(t, "[kempt]\nspec = 1\n[packages.a]\ndescription = \"a\"\nneeds = [\"ghost\"]\n")
	if len(f) != 1 || !strings.Contains(f[0].Msg, `unknown package "ghost"`) {
		t.Fatalf("got %v", f)
	}
}

func TestValidateBadOS(t *testing.T) {
	f := findingsFor(t, "[kempt]\nspec = 1\n[packages.a]\ndescription = \"a\"\nonly = { os = \"beos\" }\n")
	if len(f) != 1 {
		t.Fatalf("got %v", f)
	}
}

func TestValidateSpecVersion(t *testing.T) {
	f := findingsFor(t, "[kempt]\nspec = 2\n")
	if len(f) != 1 || !strings.Contains(f[0].Msg, "unsupported spec") {
		t.Fatalf("got %v", f)
	}
}

func TestValidateProfileRef(t *testing.T) {
	f := findingsFor(t, "[kempt]\nspec = 1\n[profiles.dev]\ndescription = \"d\"\npackages = [\"nope\"]\n")
	if len(f) != 1 {
		t.Fatalf("got %v", f)
	}
}

func TestValidateStepRequiredFields(t *testing.T) {
	f := findingsFor(t, `
[kempt]
spec = 1
[packages.a]
description = "a"
[[packages.a.symlink]]
from = "x"
`)
	if len(f) != 1 || !strings.Contains(f[0].Msg, `missing required field "to"`) {
		t.Fatalf("got %v", f)
	}
}

func TestValidateReferenceIsClean(t *testing.T) {
	m := mustParse(t, "testdata/reference.toml")
	if f := Validate(m); len(f) != 0 {
		t.Fatalf("reference should validate clean: %v", f)
	}
}
