package cli

import (
	"path/filepath"
	"strings"

	"github.com/schuettc/kempt/internal/state"
)

// splitPackages parses a comma-separated -packages flag value into a slice,
// trimming whitespace and dropping empties. Returns nil for an empty value.
func splitPackages(v string) []string {
	var packages []string
	for _, p := range strings.Split(v, ",") {
		if p = strings.TrimSpace(p); p != "" {
			packages = append(packages, p)
		}
	}
	return packages
}

// loadState returns the default store's saved state. It is a package var so
// tests can inject a state.Store backed by a tempdir.
var loadState = func() (*state.State, bool, error) {
	s, err := state.DefaultStore()
	if err != nil {
		return nil, false, err
	}
	return s.Load()
}

// saveState persists state to the default store. It is a package var so tests
// can redirect writes to a tempdir store.
var saveState = func(st *state.State) error {
	s, err := state.DefaultStore()
	if err != nil {
		return err
	}
	return s.Save(st)
}

// resolveManifest picks the manifest path. An explicit flag (non-empty) always
// wins. Otherwise, when saved state exists, the manifest lives at
// <RepoDir>/kempt.toml. With no flag and no state, fall back to "kempt.toml".
func resolveManifest(flagVal string, st *state.State, existed bool) string {
	if flagVal != "" {
		return flagVal
	}
	if existed && st != nil && st.RepoDir != "" {
		return filepath.Join(st.RepoDir, "kempt.toml")
	}
	return "kempt.toml"
}

// resolveSelection picks the profile/packages selection. Explicit flags win
// (profile XOR packages, validated downstream by engine.Select). Otherwise the
// saved selection is used. With neither, returns ("", nil) meaning all packages.
func resolveSelection(profileFlag string, packagesFlag []string, st *state.State, existed bool) (profile string, packages []string) {
	if profileFlag != "" || len(packagesFlag) > 0 {
		return profileFlag, packagesFlag
	}
	if existed && st != nil {
		return st.Profile, st.Packages
	}
	return "", nil
}
