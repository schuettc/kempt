package manifest

import (
	"strings"
	"testing"
)

func TestParseLayerHeader(t *testing.T) {
	for _, want := range []string{"", "user", "project"} {
		src := "[kempt]\nspec = 1\n"
		if want != "" {
			src += `layer = "` + want + "\"\n"
		}
		m, f := Parse([]byte(src))
		if len(f) != 0 || m.Layer != want {
			t.Errorf("layer %q: got %q, findings %v", want, m.Layer, f)
		}
	}
}

func TestValidateRejectsUnknownLayerScope(t *testing.T) {
	m, _ := Parse([]byte("[kempt]\nspec = 1\nlayer = \"machine\"\n"))
	if !hasFinding(Validate(m), "kempt.layer", "user") {
		t.Errorf("want a kempt.layer finding naming the valid scopes; got %v", Validate(m))
	}
}

// A project layer is written by whoever commits to the project, so it may only
// write files, and only inside its checkout.
func TestValidateProjectLayerLimits(t *testing.T) {
	src := `
[kempt]
spec = 1
layer = "project"
[packages.dev]
  [[packages.dev.install]]
  brew = { formulas = ["jq"] }
  [[packages.dev.symlink]]
  from = "cfg/a"
  to = "~/.a"
  [[packages.dev.json-merge]]
  file = "/etc/x.json"
  merge = { a = 1 }
  [[packages.dev.line-in-file]]
  file = "../outside"
  line = "x"
  [[packages.dev.toml-merge]]
  file = ".codex/config.toml"
  merge = { a = 1 }
  [[packages.dev.symlink]]
  from = "cfg/b"
  to = "sub/b"
`
	m, f := Parse([]byte(src))
	if len(f) != 0 {
		t.Fatal(f)
	}
	got := Validate(m)
	for _, want := range []string{
		"packages.dev.install[0]|project layer",
		"packages.dev.symlink[0]|inside the project",
		"packages.dev.json-merge[0]|inside the project",
		"packages.dev.line-in-file[0]|inside the project",
	} {
		parts := strings.SplitN(want, "|", 2)
		if !hasFinding(got, parts[0], parts[1]) {
			t.Errorf("missing finding %s; got %v", want, got)
		}
	}
	for _, f := range got {
		if strings.Contains(f.Path, "toml-merge") || strings.Contains(f.Path, "symlink[1]") {
			t.Errorf("in-project target flagged: %v", f)
		}
	}
}

// A user layer and the base keep every primitive.
func TestValidateUserLayerUnrestricted(t *testing.T) {
	m, _ := Parse([]byte("[kempt]\nspec = 1\nlayer = \"user\"\n[packages.w]\n  [[packages.w.install]]\n  brew = { formulas = [\"jq\"] }\n"))
	for _, f := range Validate(m) {
		if strings.Contains(f.Msg, "project layer") {
			t.Errorf("user layer restricted: %v", f)
		}
	}
}

func hasFinding(fs []Finding, path, msgPart string) bool {
	for _, f := range fs {
		if f.Path == path && strings.Contains(f.Msg, msgPart) {
			return true
		}
	}
	return false
}

// A layer's needs may name base packages; composition checks them, not lint.
func TestValidateLayerNeedsDeferred(t *testing.T) {
	m, _ := Parse([]byte("[kempt]\nspec = 1\nlayer = \"user\"\n[packages.a]\nneeds = [\"pi\"]\n"))
	for _, f := range Validate(m) {
		if strings.Contains(f.Path, "needs") {
			t.Errorf("layer need flagged by lint: %v", f)
		}
	}
}
