package jsonutil

import "strings"

// RemovePaths returns v without the dotted key paths in paths. A missing path
// is skipped. v is not mutated: maps along a removed path are copied.
func RemovePaths(v any, paths []string) any {
	for _, p := range paths {
		v = removePath(v, strings.Split(p, "."))
	}
	return v
}

func removePath(v any, path []string) any {
	m, ok := v.(map[string]any)
	if !ok {
		return v
	}
	child, ok := m[path[0]]
	if !ok {
		return v
	}
	out := make(map[string]any, len(m))
	for k, cv := range m {
		out[k] = cv
	}
	if len(path) == 1 {
		delete(out, path[0])
	} else {
		out[path[0]] = removePath(child, path[1:])
	}
	return out
}

// HasPath reports whether the key path exists in v.
func HasPath(v any, path []string) bool {
	for _, k := range path {
		m, ok := v.(map[string]any)
		if !ok {
			return false
		}
		if v, ok = m[k]; !ok {
			return false
		}
	}
	return true
}
