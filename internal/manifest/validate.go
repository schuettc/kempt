package manifest

import (
	"fmt"
	"sort"
	"strings"
)

// Validate performs semantic checks over a parsed Manifest, returning findings
// in a deterministic order. Rules run in the documented order (1-6).
func Validate(m *Manifest) []Finding {
	var findings []Finding
	findings = append(findings, validateSpec(m)...)
	findings = append(findings, validateNeeds(m)...)
	findings = append(findings, validateCycles(m)...)
	findings = append(findings, validateOnly(m)...)
	findings = append(findings, validateProfiles(m)...)
	findings = append(findings, validateStepFields(m)...)
	return findings
}

func sortedPackageNames(m *Manifest) []string {
	names := make([]string, 0, len(m.Packages))
	for name := range m.Packages {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Rule 1: spec must be exactly 1.
func validateSpec(m *Manifest) []Finding {
	if m.Spec != 1 {
		return []Finding{{
			Path: "kempt.spec",
			Msg:  fmt.Sprintf("unsupported spec version %d", m.Spec),
		}}
	}
	return nil
}

// Rule 2: every needs entry names an existing package.
func validateNeeds(m *Manifest) []Finding {
	var findings []Finding
	for _, name := range sortedPackageNames(m) {
		pkg := m.Packages[name]
		for _, need := range pkg.Needs {
			if _, ok := m.Packages[need]; !ok {
				findings = append(findings, Finding{
					Path: fmt.Sprintf("packages.%s.needs", name),
					Msg:  fmt.Sprintf("unknown package %q", need),
				})
			}
		}
	}
	return findings
}

// Rule 3: the needs graph is acyclic. Iterative DFS with three-color marking.
func validateCycles(m *Manifest) []Finding {
	const (
		white = 0 // unvisited
		gray  = 1 // on stack
		black = 2 // done
	)
	color := map[string]int{}
	var findings []Finding
	reported := map[string]bool{}

	for _, root := range sortedPackageNames(m) {
		if color[root] != white {
			continue
		}
		// path is the current DFS stack of package names.
		type frame struct {
			name string
			kids []string
			idx  int
		}
		var stack []frame
		pushFrame := func(name string) {
			color[name] = gray
			pkg := m.Packages[name]
			// Only follow needs that reference existing packages, in
			// sorted order for determinism.
			kids := []string{}
			for _, need := range pkg.Needs {
				if _, ok := m.Packages[need]; ok {
					kids = append(kids, need)
				}
			}
			sort.Strings(kids)
			stack = append(stack, frame{name: name, kids: kids})
		}
		pushFrame(root)
		for len(stack) > 0 {
			top := &stack[len(stack)-1]
			if top.idx >= len(top.kids) {
				color[top.name] = black
				stack = stack[:len(stack)-1]
				continue
			}
			child := top.kids[top.idx]
			top.idx++
			switch color[child] {
			case white:
				pushFrame(child)
			case gray:
				// Back-edge: build cycle path from child back to top.
				var cycle []string
				start := -1
				for i, fr := range stack {
					if fr.name == child {
						start = i
						break
					}
				}
				if start >= 0 {
					for i := start; i < len(stack); i++ {
						cycle = append(cycle, stack[i].name)
					}
					cycle = append(cycle, child)
				}
				key := strings.Join(cycle, " -> ")
				if !reported[key] {
					reported[key] = true
					findings = append(findings, Finding{
						Path: fmt.Sprintf("packages.%s", cycle[0]),
						Msg:  fmt.Sprintf("dependency cycle: %s", key),
					})
				}
			}
		}
	}
	return findings
}

var validOS = map[string]bool{"darwin": true, "linux": true, "windows": true}
var validArch = map[string]bool{"arm64": true, "amd64": true}

func onlyFindings(path string, only *Only) []Finding {
	if only == nil {
		return nil
	}
	var findings []Finding
	if only.OS != "" && !validOS[only.OS] {
		findings = append(findings, Finding{
			Path: path + ".os",
			Msg:  fmt.Sprintf("unknown os %q", only.OS),
		})
	}
	if only.Arch != "" && !validArch[only.Arch] {
		findings = append(findings, Finding{
			Path: path + ".arch",
			Msg:  fmt.Sprintf("unknown arch %q", only.Arch),
		})
	}
	return findings
}

// stepOnly extracts the Only pointer from a step via type switch.
func stepOnly(s Step) *Only {
	switch v := s.(type) {
	case InstallStep:
		return v.Only
	case GithubReleaseStep:
		return v.Only
	case GitCloneStep:
		return v.Only
	case ServiceStep:
		return v.Only
	case SymlinkStep:
		return v.Only
	case JSONMergeStep:
		return v.Only
	case LineInFileStep:
		return v.Only
	case VerifyStep:
		return v.Only
	}
	return nil
}

// Rule 4: only.os and only.arch use known values (package- and step-level).
func validateOnly(m *Manifest) []Finding {
	var findings []Finding
	for _, name := range sortedPackageNames(m) {
		pkg := m.Packages[name]
		findings = append(findings, onlyFindings(fmt.Sprintf("packages.%s.only", name), pkg.Only)...)
		kindIdx := map[string]int{}
		for _, step := range pkg.Steps {
			kind := step.Kind()
			idx := kindIdx[kind]
			path := fmt.Sprintf("packages.%s.%s[%d].only", name, kind, idx)
			findings = append(findings, onlyFindings(path, stepOnly(step))...)
			kindIdx[kind]++
		}
	}
	return findings
}

// Rule 5: every profile package exists.
func validateProfiles(m *Manifest) []Finding {
	var findings []Finding
	names := make([]string, 0, len(m.Profiles))
	for name := range m.Profiles {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		pr := m.Profiles[name]
		for _, p := range pr.Packages {
			if _, ok := m.Packages[p]; !ok {
				findings = append(findings, Finding{
					Path: fmt.Sprintf("profiles.%s", name),
					Msg:  fmt.Sprintf("unknown package %q", p),
				})
			}
		}
	}
	return findings
}

// missingFields returns the names of required fields absent from a step.
func missingFields(s Step) []string {
	var missing []string
	req := func(cond bool, field string) {
		if !cond {
			missing = append(missing, field)
		}
	}
	switch v := s.(type) {
	case SymlinkStep:
		req(v.From != "", "from")
		req(v.To != "", "to")
	case GithubReleaseStep:
		req(v.Repo != "", "repo")
		req(v.Asset != "", "asset")
		req(v.Bin != "", "bin")
	case GitCloneStep:
		req(v.Repo != "", "repo")
		req(v.To != "", "to")
	case ServiceStep:
		req(v.Label != "", "label")
		req(len(v.Program) > 0, "program")
	case JSONMergeStep:
		req(v.File != "", "file")
		req(len(v.Merge) > 0, "merge")
	case LineInFileStep:
		req(v.File != "", "file")
		req(v.Line != "", "line")
	case InstallStep:
		brewHasContent := v.Brew != nil && (len(v.Brew.Formulas) > 0 || len(v.Brew.Casks) > 0 || len(v.Brew.Taps) > 0)
		hasBackend := brewHasContent || len(v.Winget) > 0 || len(v.Apt) > 0
		req(hasBackend, "install")
	case VerifyStep:
		hasCheck := v.CommandExists != "" || v.SymlinkTarget != nil || v.VersionCurrent != nil
		req(hasCheck, "verify")
	}
	return missing
}

// Rule 6: per-step required fields.
func validateStepFields(m *Manifest) []Finding {
	var findings []Finding
	for _, name := range sortedPackageNames(m) {
		pkg := m.Packages[name]
		kindIdx := map[string]int{}
		for _, step := range pkg.Steps {
			kind := step.Kind()
			idx := kindIdx[kind]
			for _, field := range missingFields(step) {
				findings = append(findings, Finding{
					Path: fmt.Sprintf("packages.%s.%s[%d]", name, kind, idx),
					Msg:  fmt.Sprintf("missing required field %q", field),
				})
			}
			kindIdx[kind]++
		}
	}
	return findings
}
