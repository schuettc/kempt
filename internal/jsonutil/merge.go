package jsonutil

import (
	"encoding/json"
	"strings"
)

// ExpandHome walks a decoded value tree and replaces the literal token
// "${HOME}" in every string leaf with home. json-merge and toml-merge apply it
// to desired values so a manifest can write an absolute home path into files
// that do not themselves expand ~ or environment variables (e.g. codex
// hooks.json, whose command strings need absolute paths). Bare ~ is left
// untouched — consumers that expand it at runtime (claude, tmux) keep doing so.
func ExpandHome(v any, home string) any {
	switch t := v.(type) {
	case map[string]any:
		for k, e := range t {
			t[k] = ExpandHome(e, home)
		}
		return t
	case []any:
		for i, e := range t {
			t[i] = ExpandHome(e, home)
		}
		return t
	case string:
		return strings.ReplaceAll(t, "${HOME}", home)
	default:
		return v
	}
}

// ToAny round-trips a map through JSON so its values (which may come from TOML)
// share the same dynamic types as unmarshalled current state, making deep
// comparison reliable.
func ToAny(m map[string]any) any {
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
