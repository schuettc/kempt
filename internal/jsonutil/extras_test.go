package jsonutil

import (
	"reflect"
	"testing"
)

func TestArrayExtras(t *testing.T) {
	desired := []any{"npm:a", "npm:b"}
	current := []any{"npm:a", "npm:a@1.0.0", "npm:b", "npm:c"}
	got := ArrayExtras(desired, current)
	want := []any{"npm:a@1.0.0", "npm:c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ArrayExtras = %v; want %v", got, want)
	}
	if e := ArrayExtras([]any{"x"}, []any{"x"}); len(e) != 0 {
		t.Fatalf("no extras expected, got %v", e)
	}
}
