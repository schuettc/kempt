package doctor

import (
	"path/filepath"
	"testing"
)

func TestD2FlagsDuplicateIdentity(t *testing.T) {
	ctx := ctxT(t)
	f := filepath.Join(t.TempDir(), "settings.json")
	writeFile(t, f, `{"packages":["npm:pi-quiet","npm:pi-quiet@0.2.0","npm:solo"]}`)
	pkgs := pkgWithMerge(f, map[string]any{"packages": []any{"npm:pi-quiet", "npm:solo"}}, "")
	got := CheckDupIdentity(ctx, pkgs)
	if len(got) != 1 || got[0].Severity != Warn {
		t.Fatalf("want 1 warn for pi-quiet, got %+v", got)
	}
}

func TestD2NoDuplicates(t *testing.T) {
	ctx := ctxT(t)
	f := filepath.Join(t.TempDir(), "settings.json")
	writeFile(t, f, `{"packages":["npm:a","npm:b"]}`)
	if got := CheckDupIdentity(ctx, pkgWithMerge(f, map[string]any{"packages": []any{"npm:a", "npm:b"}}, "")); len(got) != 0 {
		t.Fatalf("want none, got %+v", got)
	}
}
