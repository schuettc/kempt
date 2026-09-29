package manifest

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Layer scopes. A base manifest has no scope.
const (
	LayerUser    = "user"
	LayerProject = "project"
)

// validateLayer checks the [kempt] layer scope and, for a project layer, that
// it only writes files and only inside its checkout: a project layer is
// written by whoever commits to the project, so it must not install software,
// run services, or reach the user's home.
func validateLayer(m *Manifest) []Finding {
	switch m.Layer {
	case "", LayerUser:
		return nil
	case LayerProject:
	default:
		return []Finding{{Path: "kempt.layer", Msg: fmt.Sprintf("unknown layer scope %q (want %q or %q)", m.Layer, LayerUser, LayerProject)}}
	}
	var findings []Finding
	for _, name := range sortedPackageNames(m) {
		idx := map[string]int{}
		for _, step := range m.Packages[name].Steps {
			kind := step.Kind()
			path := fmt.Sprintf("packages.%s.%s[%d]", name, kind, idx[kind])
			idx[kind]++
			target, filesStep := projectTarget(step)
			switch {
			case kind == "verify":
			case !filesStep:
				findings = append(findings, Finding{Path: path, Msg: "a project layer may only use symlink, json-merge, toml-merge, line-in-file and verify"})
			case !insideProject(target):
				findings = append(findings, Finding{Path: path, Msg: fmt.Sprintf("target %q must be a relative path inside the project checkout", target)})
			}
		}
	}
	return findings
}

// projectTarget returns the path a files-class step writes, and whether the
// step is one a project layer may use.
func projectTarget(step Step) (string, bool) {
	switch s := step.(type) {
	case SymlinkStep:
		return s.To, true
	case JSONMergeStep:
		return s.File, true
	case TomlMergeStep:
		return s.File, true
	case LineInFileStep:
		return s.File, true
	}
	return "", false
}

// insideProject reports whether p is relative and stays within its root: no
// "~", no absolute path, no ".." escape.
func insideProject(p string) bool {
	if p == "" || p == "~" || strings.HasPrefix(p, "~/") || filepath.IsAbs(p) {
		return false
	}
	clean := filepath.Clean(p)
	return clean != ".." && !strings.HasPrefix(clean, "../")
}
