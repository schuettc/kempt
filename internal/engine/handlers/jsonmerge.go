package handlers

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/schuettc/kempt/internal/engine"
	"github.com/schuettc/kempt/internal/machine"
	"github.com/schuettc/kempt/internal/manifest"
)

func init() { engine.RegisterHandler(jsonMergeHandler{}) }

// jsonMergeHandler realises the desired state: ctx.Expand(File) is JSON that is
// a deep superset of Merge. Maps recurse, arrays gain missing elements, scalars
// overwrite.
type jsonMergeHandler struct{}

func (jsonMergeHandler) Kind() string { return "json-merge" }

func (jsonMergeHandler) Inspect(ctx *machine.Context, s manifest.Step) (engine.Delta, error) {
	st := s.(manifest.JSONMergeStep)
	file := ctx.Expand(st.File)
	base := "json-merge " + file

	b, err := os.ReadFile(file)
	if err != nil {
		if os.IsNotExist(err) {
			return engine.Delta{Op: engine.OpChange, Detail: base + " (create file)"}, nil
		}
		return engine.Delta{}, err
	}

	var current any
	if err := json.Unmarshal(b, &current); err != nil {
		return engine.Delta{Op: engine.OpBlocked, Detail: base + " (existing file is not valid JSON)"}, nil
	}

	desired := expandHome(toAny(st.Merge), ctx.Home)
	replace := st.Arrays == "replace"
	if isSubset(desired, current, replace) {
		return engine.Delta{Op: engine.OpNoop, Detail: base}, nil
	}

	keys := mergeKeys(desired.(map[string]any), current, replace)
	return engine.Delta{Op: engine.OpChange, Detail: base + fmt.Sprintf(" (merge keys: %s)", strings.Join(keys, ", "))}, nil
}

func (jsonMergeHandler) Apply(ctx *machine.Context, s manifest.Step) error {
	st := s.(manifest.JSONMergeStep)
	file := ctx.Expand(st.File)

	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}

	var current any
	b, err := os.ReadFile(file)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		current = map[string]any{}
	} else if err := json.Unmarshal(b, &current); err != nil {
		return fmt.Errorf("existing file is not valid JSON: %s", file)
	}

	merged := merge(expandHome(toAny(st.Merge), ctx.Home), current, st.Arrays == "replace")
	out, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(file, append(out, '\n'), 0o644)
}

// mergeKeys returns the sorted top-level desired keys whose values are not
// already a deep subset of current.
func mergeKeys(desired map[string]any, current any, replaceArrays bool) []string {
	curMap, _ := current.(map[string]any)
	var keys []string
	for k, dv := range desired {
		cv, ok := curMap[k]
		if !ok || !isSubset(dv, cv, replaceArrays) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys
}

// isSubset reports whether desired is deeply contained in current: maps recurse
// per key, scalars must be equal. Array handling depends on replaceArrays: when
// false (append mode) every desired element must be present (deep equality);
// when true (replace mode) the desired array must deep-equal current exactly, so
// any extra, reordered, or differing element drives a change.
func isSubset(desired, current any, replaceArrays bool) bool {
	switch d := desired.(type) {
	case map[string]any:
		c, ok := current.(map[string]any)
		if !ok {
			return false
		}
		for k, dv := range d {
			cv, ok := c[k]
			if !ok || !isSubset(dv, cv, replaceArrays) {
				return false
			}
		}
		return true
	case []any:
		c, ok := current.([]any)
		if !ok {
			return false
		}
		if replaceArrays {
			return reflect.DeepEqual(d, c)
		}
		for _, de := range d {
			if !containsElem(c, de) {
				return false
			}
		}
		return true
	default:
		return reflect.DeepEqual(desired, current)
	}
}

// merge returns the deep merge of desired into current: maps recurse, scalars
// and mismatched types take the desired value. Array handling depends on
// replaceArrays: when false (append mode) desired elements missing from current
// are appended (order preserved); when true (replace mode) the desired array
// replaces the current array wholesale.
func merge(desired, current any, replaceArrays bool) any {
	switch d := desired.(type) {
	case map[string]any:
		c, ok := current.(map[string]any)
		if !ok {
			c = map[string]any{}
		}
		out := map[string]any{}
		for k, v := range c {
			out[k] = v
		}
		for k, dv := range d {
			if cv, ok := out[k]; ok {
				out[k] = merge(dv, cv, replaceArrays)
			} else {
				out[k] = dv
			}
		}
		return out
	case []any:
		if replaceArrays {
			return append([]any{}, d...)
		}
		c, ok := current.([]any)
		if !ok {
			return append([]any{}, d...)
		}
		out := append([]any{}, c...)
		for _, de := range d {
			if !containsElem(out, de) {
				out = append(out, de)
			}
		}
		return out
	default:
		return desired
	}
}

func containsElem(list []any, elem any) bool {
	for _, e := range list {
		if reflect.DeepEqual(e, elem) {
			return true
		}
	}
	return false
}

// expandHome walks a decoded value tree and replaces the literal token
// "${HOME}" in every string leaf with home. json-merge and toml-merge apply it
// to desired values so a manifest can write an absolute home path into files
// that do not themselves expand ~ or environment variables (e.g. codex
// hooks.json, whose command strings need absolute paths). Bare ~ is left
// untouched — consumers that expand it at runtime (claude, tmux) keep doing so.
func expandHome(v any, home string) any {
	switch t := v.(type) {
	case map[string]any:
		for k, e := range t {
			t[k] = expandHome(e, home)
		}
		return t
	case []any:
		for i, e := range t {
			t[i] = expandHome(e, home)
		}
		return t
	case string:
		return strings.ReplaceAll(t, "${HOME}", home)
	default:
		return v
	}
}

// toAny round-trips a map through JSON so its values (which may come from TOML)
// share the same dynamic types as unmarshalled current state, making deep
// comparison reliable.
func toAny(m map[string]any) any {
	b, err := json.Marshal(m)
	if err != nil {
		return m
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return m
	}
	return v
}
