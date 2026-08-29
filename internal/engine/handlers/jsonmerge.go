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

	desired := toAny(st.Merge)
	if isSubset(desired, current) {
		return engine.Delta{Op: engine.OpNoop, Detail: base}, nil
	}

	keys := mergeKeys(st.Merge, current)
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

	merged := merge(toAny(st.Merge), current)
	out, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(file, append(out, '\n'), 0o644)
}

// mergeKeys returns the sorted top-level desired keys whose values are not
// already a deep subset of current.
func mergeKeys(desired map[string]any, current any) []string {
	curMap, _ := current.(map[string]any)
	var keys []string
	for k, dv := range desired {
		cv, ok := curMap[k]
		if !ok || !isSubset(dv, cv) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys
}

// isSubset reports whether desired is deeply contained in current: maps recurse
// per key, arrays require every desired element to be present (deep equality),
// scalars must be equal.
func isSubset(desired, current any) bool {
	switch d := desired.(type) {
	case map[string]any:
		c, ok := current.(map[string]any)
		if !ok {
			return false
		}
		for k, dv := range d {
			cv, ok := c[k]
			if !ok || !isSubset(dv, cv) {
				return false
			}
		}
		return true
	case []any:
		c, ok := current.([]any)
		if !ok {
			return false
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

// merge returns the deep merge of desired into current: maps recurse, arrays
// append desired elements missing from current (order preserved), scalars and
// mismatched types take the desired value.
func merge(desired, current any) any {
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
				out[k] = merge(dv, cv)
			} else {
				out[k] = dv
			}
		}
		return out
	case []any:
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
