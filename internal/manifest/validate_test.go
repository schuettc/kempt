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
	if len(f) != 1 || !strings.Contains(f[0].Msg, "cycle") {
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

func TestValidateStepFindingUsesPerKindIndex(t *testing.T) {
	f := findingsFor(t, `
[kempt]
spec = 1
[packages.a]
description = "a"
[[packages.a.install]]
brew = { formulas = ["jq"] }
[[packages.a.symlink]]
from = "one"
to = "~/one"
[[packages.a.symlink]]
from = "two"
`)
	if len(f) != 1 || !strings.Contains(f[0].Path, "symlink[1]") {
		t.Fatalf("want path containing symlink[1] (per-kind index), got %v", f)
	}
}

func TestValidateEmptyBrewTableIsNotABackend(t *testing.T) {
	f := findingsFor(t, `
[kempt]
spec = 1
[packages.a]
description = "a"
[[packages.a.install]]
brew = {}
`)
	if len(f) != 1 {
		t.Fatalf("brew={} must fail the >=1-backend check, got %v", f)
	}
}

func TestValidateStepLevelOnlyBadArch(t *testing.T) {
	f := findingsFor(t, `
[kempt]
spec = 1
[packages.a]
description = "a"
[[packages.a.symlink]]
from = "x"
to = "~/x"
only = { arch = "sparc" }
`)
	if len(f) != 1 || !strings.Contains(f[0].Msg, `unknown arch "sparc"`) {
		t.Fatalf("got %v", f)
	}
}

func TestValidateMissingFieldGithubReleaseBin(t *testing.T) {
	f := findingsFor(t, `
[kempt]
spec = 1
[packages.a]
description = "a"
[[packages.a.github-release]]
repo = "owner/repo"
asset = "asset.tar.gz"
`)
	if len(f) != 1 || !strings.Contains(f[0].Path, "github-release[0]") || !strings.Contains(f[0].Msg, `missing required field "bin"`) {
		t.Fatalf("got %v", f)
	}
}

func TestValidateMissingFieldGitCloneTo(t *testing.T) {
	f := findingsFor(t, `
[kempt]
spec = 1
[packages.a]
description = "a"
[[packages.a.git-clone]]
repo = "https://example.com/repo"
`)
	if len(f) != 1 || !strings.Contains(f[0].Path, "git-clone[0]") || !strings.Contains(f[0].Msg, `missing required field "to"`) {
		t.Fatalf("got %v", f)
	}
}

func TestValidateMissingFieldServiceProgram(t *testing.T) {
	f := findingsFor(t, `
[kempt]
spec = 1
[packages.a]
description = "a"
[[packages.a.service]]
label = "com.example.svc"
`)
	if len(f) != 1 || !strings.Contains(f[0].Path, "service[0]") || !strings.Contains(f[0].Msg, `missing required field "program"`) {
		t.Fatalf("got %v", f)
	}
}

func TestValidateMissingFieldJSONMergeMerge(t *testing.T) {
	f := findingsFor(t, `
[kempt]
spec = 1
[packages.a]
description = "a"
[[packages.a.json-merge]]
file = "/tmp/foo.json"
`)
	if len(f) != 1 || !strings.Contains(f[0].Path, "json-merge[0]") || !strings.Contains(f[0].Msg, `missing required field "merge"`) {
		t.Fatalf("got %v", f)
	}
}

func TestValidateMissingFieldLineInFileLine(t *testing.T) {
	f := findingsFor(t, `
[kempt]
spec = 1
[packages.a]
description = "a"
[[packages.a.line-in-file]]
file = "/etc/hosts"
`)
	if len(f) != 1 || !strings.Contains(f[0].Path, "line-in-file[0]") || !strings.Contains(f[0].Msg, `missing required field "line"`) {
		t.Fatalf("got %v", f)
	}
}

func TestValidateMissingFieldVerifyNoChecks(t *testing.T) {
	f := findingsFor(t, `
[kempt]
spec = 1
[packages.a]
description = "a"
[[packages.a.verify]]
`)
	if len(f) != 1 || !strings.Contains(f[0].Path, "verify[0]") || !strings.Contains(f[0].Msg, `missing required field`) {
		t.Fatalf("got %v", f)
	}
}

func TestValidateReferenceIsClean(t *testing.T) {
	m := mustParse(t, "testdata/reference.toml")
	if f := Validate(m); len(f) != 0 {
		t.Fatalf("reference should validate clean: %v", f)
	}
}
