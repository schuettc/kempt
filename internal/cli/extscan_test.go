package cli

import (
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
