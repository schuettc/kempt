// Package jsonutil holds small pure helpers for comparing manifest-desired
// JSON against live files, shared by json-merge inspection and doctor checks.
package jsonutil

import "strings"

// SpecIdentity collapses a kempt package spec to its identity: scheme and
// version/ref stripped, so "npm:pi-quiet@0.2.0" and "npm:pi-quiet" match. An
// @scope/name keeps its leading @; only a trailing @segment is a version/ref.
func SpecIdentity(spec string) string {
	s := spec
	switch {
	case strings.HasPrefix(s, "npm:"):
		s = strings.TrimPrefix(s, "npm:")
		if at := strings.LastIndex(s, "@"); at > 0 { // >0 keeps @scope
			s = s[:at]
		}
	case strings.HasPrefix(s, "git:"):
		s = strings.TrimPrefix(s, "git:")
		if at := strings.LastIndex(s, "@"); at > 0 {
			s = s[:at]
		}
	}
	return s
}
