package handlers

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/schuettc/kempt/internal/engine"
	"github.com/schuettc/kempt/internal/manifest"
)

func jsonHandler(t *testing.T) engine.Handler {
	t.Helper()
	h, ok := engine.HandlerFor("json-merge")
	if !ok {
		t.Fatal("json-merge handler not registered")
	}
	return h
}

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readJSON(t *testing.T, path string) any {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestJSONMergeSubsetNoop(t *testing.T) {
	h := jsonHandler(t)
	ctx := testCtx(t)
	f := filepath.Join(ctx.RepoDir, "c.json")
	writeJSON(t, f, map[string]any{"a": 1.0, "b": map[string]any{"c": 2.0}})
	s := manifest.JSONMergeStep{File: f, Merge: map[string]any{"b": map[string]any{"c": 2.0}}}
	d, err := h.Inspect(ctx, s)
	if err != nil {
		t.Fatal(err)
	}
	if d.Op != engine.OpNoop {
		t.Fatalf("op = %v (%q), want noop", d.Op, d.Detail)
	}
}

func TestJSONMergeNestedMapPreservesSiblings(t *testing.T) {
	h := jsonHandler(t)
	ctx := testCtx(t)
	f := filepath.Join(ctx.RepoDir, "c.json")
	writeJSON(t, f, map[string]any{"b": map[string]any{"keep": 1.0}})
	s := manifest.JSONMergeStep{File: f, Merge: map[string]any{"b": map[string]any{"add": 2.0}}}

	d, err := h.Inspect(ctx, s)
	if err != nil {
		t.Fatal(err)
	}
	if d.Op != engine.OpChange || !strings.Contains(d.Detail, "merge keys: b") {
		t.Fatalf("got %v %q, want change merge keys: b", d.Op, d.Detail)
	}
	assertNoopAfterApply(t, h, ctx, s)

	got := readJSON(t, f)
	want := map[string]any{"b": map[string]any{"keep": 1.0, "add": 2.0}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestJSONMergeArrayEnsureElement(t *testing.T) {
	h := jsonHandler(t)

	t.Run("already present is noop", func(t *testing.T) {
		ctx := testCtx(t)
		f := filepath.Join(ctx.RepoDir, "c.json")
		writeJSON(t, f, map[string]any{"list": []any{"a", "b"}})
		s := manifest.JSONMergeStep{File: f, Merge: map[string]any{"list": []any{"b"}}}
		d, err := h.Inspect(ctx, s)
		if err != nil {
			t.Fatal(err)
		}
		if d.Op != engine.OpNoop {
			t.Fatalf("op = %v (%q), want noop", d.Op, d.Detail)
		}
	})

	t.Run("missing element appended once", func(t *testing.T) {
		ctx := testCtx(t)
		f := filepath.Join(ctx.RepoDir, "c.json")
		writeJSON(t, f, map[string]any{"list": []any{"a"}})
		s := manifest.JSONMergeStep{File: f, Merge: map[string]any{"list": []any{"b"}}}
		assertNoopAfterApply(t, h, ctx, s)
		got := readJSON(t, f)
		want := map[string]any{"list": []any{"a", "b"}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %v, want %v", got, want)
		}
	})
}

func TestJSONMergeScalarOverwrite(t *testing.T) {
	h := jsonHandler(t)
	ctx := testCtx(t)
	f := filepath.Join(ctx.RepoDir, "c.json")
	writeJSON(t, f, map[string]any{"a": 1.0})
	s := manifest.JSONMergeStep{File: f, Merge: map[string]any{"a": 2.0}}
	assertNoopAfterApply(t, h, ctx, s)
	got := readJSON(t, f)
	want := map[string]any{"a": 2.0}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestJSONMergeInvalidJSONBlocked(t *testing.T) {
	h := jsonHandler(t)
	ctx := testCtx(t)
	f := filepath.Join(ctx.RepoDir, "c.json")
	if err := os.WriteFile(f, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := manifest.JSONMergeStep{File: f, Merge: map[string]any{"a": 1.0}}
	d, err := h.Inspect(ctx, s)
	if err != nil {
		t.Fatal(err)
	}
	if d.Op != engine.OpBlocked || !strings.Contains(d.Detail, "not valid JSON") {
		t.Fatalf("got %v %q, want blocked invalid JSON", d.Op, d.Detail)
	}
}

func TestJSONMergeCreateMissingFile(t *testing.T) {
	h := jsonHandler(t)
	ctx := testCtx(t)
	f := filepath.Join(ctx.RepoDir, "nested", "c.json")
	s := manifest.JSONMergeStep{File: f, Merge: map[string]any{"a": 1.0}}
	d, err := h.Inspect(ctx, s)
	if err != nil {
		t.Fatal(err)
	}
	if d.Op != engine.OpChange || !strings.Contains(d.Detail, "(create file)") {
		t.Fatalf("got %v %q, want change create file", d.Op, d.Detail)
	}
	assertNoopAfterApply(t, h, ctx, s)
	got := readJSON(t, f)
	want := map[string]any{"a": 1.0}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	fi, err := os.Stat(f)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o644 {
		t.Fatalf("mode = %v, want 0644", fi.Mode().Perm())
	}
	// trailing newline
	b, _ := os.ReadFile(f)
	if len(b) == 0 || b[len(b)-1] != '\n' {
		t.Fatalf("want trailing newline, got %q", string(b))
	}
}

func TestIsSubset(t *testing.T) {
	tests := []struct {
		name         string
		desired, cur any
		want         bool
	}{
		{"scalar equal", 1.0, 1.0, true},
		{"scalar differ", 1.0, 2.0, false},
		{"map subset", map[string]any{"a": 1.0}, map[string]any{"a": 1.0, "b": 2.0}, true},
		{"map missing key", map[string]any{"c": 1.0}, map[string]any{"a": 1.0}, false},
		{"nested map subset", map[string]any{"a": map[string]any{"b": 1.0}}, map[string]any{"a": map[string]any{"b": 1.0, "c": 2.0}}, true},
		{"array elements present", []any{"b"}, []any{"a", "b"}, true},
		{"array element missing", []any{"z"}, []any{"a", "b"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isSubset(tt.desired, tt.cur); got != tt.want {
				t.Fatalf("isSubset = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMerge(t *testing.T) {
	tests := []struct {
		name               string
		desired, cur, want any
	}{
		{"scalar overwrite", 2.0, 1.0, 2.0},
		{"map recurse", map[string]any{"a": 2.0}, map[string]any{"a": 1.0, "b": 3.0}, map[string]any{"a": 2.0, "b": 3.0}},
		{"array append missing", []any{"b", "c"}, []any{"a", "b"}, []any{"a", "b", "c"}},
		{"nested map", map[string]any{"x": map[string]any{"y": 2.0}}, map[string]any{"x": map[string]any{"z": 1.0}}, map[string]any{"x": map[string]any{"y": 2.0, "z": 1.0}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := merge(tt.desired, tt.cur); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("merge = %v, want %v", got, tt.want)
			}
		})
	}
}
