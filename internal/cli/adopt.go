package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/schuettc/kempt/internal/engine"
	"github.com/schuettc/kempt/internal/manifest"
	"github.com/schuettc/kempt/internal/state"
)

func init() {
	Register(Command{Name: "adopt", Summary: "add a package (and its needs) to the saved selection", Run: runAdopt})
	Register(Command{Name: "drop", Summary: "remove a package from the saved selection", Run: runDrop})
}

// parseOnePositional parses args with an empty flag set (so -h is handled) and
// requires exactly one positional argument.
func parseOnePositional(name string, args []string, out io.Writer) (string, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	if err := ParseFlags(fs, args, out); err != nil {
		return "", err
	}
	rest := fs.Args()
	if len(rest) != 1 {
		return "", UsageError{Msg: fmt.Sprintf("usage: kempt %s <pkg>", name)}
	}
	return rest[0], nil
}

// loadStateManifest loads the saved state (must exist) and its manifest.
func loadStateManifest() (*state.State, *manifest.Manifest, error) {
	st, existed, err := loadState()
	if err != nil {
		return nil, nil, err
	}
	if !existed {
		return nil, nil, UsageError{Msg: "no saved selection; run kempt init first"}
	}
	manifestPath := filepath.Join(st.RepoDir, "kempt.toml")
	src, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, nil, UsageError{Msg: fmt.Sprintf("cannot read %s: %v", manifestPath, err)}
	}
	m, findings := manifest.Parse(src)
	if m != nil {
		findings = append(findings, manifest.Validate(m)...)
	}
	if len(findings) > 0 {
		return nil, nil, fmt.Errorf("manifest has findings; run kempt lint")
	}
	return st, m, nil
}

func runAdopt(args []string, out, errw io.Writer) error {
	pkg, err := parseOnePositional("adopt", args, out)
	if err != nil {
		return err
	}
	st, m, err := loadStateManifest()
	if err != nil {
		return err
	}
	if _, ok := m.Packages[pkg]; !ok {
		return UsageError{Msg: fmt.Sprintf("unknown package %q", pkg)}
	}

	// Needs-closure of the adopted package.
	selected, err := engine.Select(m, "", []string{pkg})
	if err != nil {
		return UsageError{Msg: err.Error()}
	}

	// Union the closure names into the saved selection, deduped and sorted.
	set := map[string]bool{}
	for _, p := range st.Packages {
		set[p] = true
	}
	var addedDeps []string
	for _, p := range selected {
		if p.Name == pkg {
			continue
		}
		if !set[p.Name] {
			addedDeps = append(addedDeps, p.Name)
		}
	}
	for _, p := range selected {
		set[p.Name] = true
	}
	names := make([]string, 0, len(set))
	for n := range set {
		names = append(names, n)
	}
	sort.Strings(names)
	st.Packages = names

	if err := saveState(st); err != nil {
		return err
	}

	sort.Strings(addedDeps)
	if len(addedDeps) > 0 {
		fmt.Fprintf(out, "adopted %s (+ deps: %s)\n", pkg, joinComma(addedDeps))
	} else {
		fmt.Fprintf(out, "adopted %s\n", pkg)
	}
	fmt.Fprintln(out, "run kempt apply to converge")
	return nil
}

func runDrop(args []string, out, errw io.Writer) error {
	pkg, err := parseOnePositional("drop", args, out)
	if err != nil {
		return err
	}
	st, m, err := loadStateManifest()
	if err != nil {
		return err
	}

	// Refuse if the package is not in the current selection.
	inSelection := false
	for _, name := range st.Packages {
		if name == pkg {
			inSelection = true
			break
		}
	}
	if !inSelection {
		return UsageError{Msg: fmt.Sprintf("%q is not in the current selection", pkg)}
	}

	// Refuse if any still-selected package (other than pkg) needs pkg.
	var needers []string
	for _, name := range st.Packages {
		if name == pkg {
			continue
		}
		p, ok := m.Packages[name]
		if !ok {
			continue
		}
		for _, need := range p.Needs {
			if need == pkg {
				needers = append(needers, name)
				break
			}
		}
	}
	if len(needers) > 0 {
		sort.Strings(needers)
		return UsageError{Msg: fmt.Sprintf("cannot drop %s: still needed by %s", pkg, joinComma(needers))}
	}

	var kept []string
	for _, name := range st.Packages {
		if name != pkg {
			kept = append(kept, name)
		}
	}
	st.Packages = kept
	if err := saveState(st); err != nil {
		return err
	}
	fmt.Fprintf(out, "dropped %s\n", pkg)
	return nil
}

func joinComma(s []string) string {
	out := ""
	for i, v := range s {
		if i > 0 {
			out += ", "
		}
		out += v
	}
	return out
}
