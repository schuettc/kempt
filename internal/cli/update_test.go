package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRollRollingRollsBehindExtension(t *testing.T) {
	dir := writeTempManifest(t, `
[kempt]
spec = 1
[packages.pi]
  [[packages.pi.install]]
  pi = ["npm:pi-creel", "npm:pi-quiet@0.2.0"]
`)
	restore, fr := stubContextRuns(t, dir, nil, map[string]string{
		"pi list":                   "  npm:pi-creel@0.1.0\n  npm:pi-quiet@0.2.0\n",
		"npm view pi-creel version": "0.1.1\n",
		"pi install npm:pi-creel":   "",
	}, nil)
	defer restore()

	_, selected, ctx, err := loadSelectedContext(dir+"/kempt.toml", "", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := rollRolling(ctx, selected, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "rolled pi-creel to 0.1.1") {
		t.Errorf("missing roll line:\n%s", out.String())
	}
	rolled := false
	for _, c := range fr.Calls {
		if c == "pi install npm:pi-creel" {
			rolled = true
		}
		if c == "pi install npm:pi-quiet@0.2.0" {
			t.Errorf("pinned entry must not be rolled; calls=%v", fr.Calls)
		}
	}
	if !rolled {
		t.Errorf("rolling entry not rolled; calls=%v", fr.Calls)
	}
}
