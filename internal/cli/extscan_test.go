package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestScanExtensionsReportsBehind(t *testing.T) {
	dir := writeTempManifest(t, `
[kempt]
spec = 1
[packages.pi]
  [[packages.pi.install]]
  pi = ["npm:pi-creel", "npm:pi-quiet@0.2.0"]
`)
	restore, _ := stubContextRuns(t, dir, nil, map[string]string{
		"pi list":                   "  npm:pi-creel@0.1.0\n  npm:pi-quiet@0.2.0\n",
		"npm view pi-creel version": "0.1.1\n",
	}, nil)
	defer restore()

	_, selected, ctx, err := loadSelectedContext(dir+"/kempt.toml", "", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := scanExtensions(ctx, selected)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 rolling status (pinned excluded), got %d: %+v", len(got), got)
	}
	s := got[0]
	if s.Kind != "pi" || s.Tool != "pi-creel" || s.Installed != "0.1.0" || s.Target != "0.1.1" || !s.Behind {
		t.Errorf("status = %+v", s)
	}
}

func TestOutdatedIncludesExtensions(t *testing.T) {
	dir := writeTempManifest(t, `
[kempt]
spec = 1
[packages.pi]
  [[packages.pi.install]]
  pi = ["npm:pi-creel"]
`)
	restore, _ := stubContextRuns(t, dir, nil, map[string]string{
		"pi list":                   "  npm:pi-creel@0.1.0\n",
		"npm view pi-creel version": "0.1.1\n",
	}, nil)
	defer restore()

	var out bytes.Buffer
	if err := runOutdated([]string{"-manifest", dir + "/kempt.toml"}, &out, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "pi-creel  0.1.0 -> 0.1.1") {
		t.Errorf("outdated output missing extension line:\n%s", out.String())
	}
}

func TestUpgradeRollsExtension(t *testing.T) {
	dir := writeTempManifest(t, `
[kempt]
spec = 1
[packages.pi]
  [[packages.pi.install]]
  pi = ["npm:pi-creel"]
`)
	restore, fr := stubContextRuns(t, dir, nil, map[string]string{
		"pi list":                   "  npm:pi-creel@0.1.0\n",
		"npm view pi-creel version": "0.1.1\n",
		"pi install npm:pi-creel":   "",
	}, nil)
	defer restore()

	var out bytes.Buffer
	if err := runUpgrade([]string{"-yes", "-manifest", dir + "/kempt.toml"}, &out, &out); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range fr.Calls {
		if c == "pi install npm:pi-creel" {
			found = true
		}
	}
	if !found {
		t.Errorf("upgrade did not run `pi install npm:pi-creel`; calls=%v", fr.Calls)
	}
	if !strings.Contains(out.String(), "upgraded pi-creel to 0.1.1") {
		t.Errorf("upgrade output missing confirmation:\n%s", out.String())
	}
}

func TestScanExtensionsReportsGitBehind(t *testing.T) {
	dir := writeTempManifest(t, `
[kempt]
spec = 1
[packages.pi]
  [[packages.pi.install]]
  pi = ["git:github.com/obra/superpowers"]
`)
	restore, _ := stubContextRuns(t, dir, nil, map[string]string{
		"pi list":                     "  git:github.com/obra/superpowers\n    /h/sp\n",
		"git -C /h/sp rev-parse HEAD": "5bf4e78aaaaaaaaaaaaa\n",
		"git ls-remote https://github.com/obra/superpowers HEAD": "9c01d2bbbbbbbbbbbbbb\tHEAD\n",
	}, nil)
	defer restore()

	_, selected, ctx, err := loadSelectedContext(dir+"/kempt.toml", "", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := scanExtensions(ctx, selected)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 rolling status, got %d: %+v", len(got), got)
	}
	s := got[0]
	if s.Kind != "git" || s.Tool != "github.com/obra/superpowers" || s.Installed != "5bf4e78aaaaa" || s.Target != "9c01d2bbbbbb" || !s.Behind {
		t.Errorf("status = %+v", s)
	}
}
