package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/schuettc/kempt/internal/engine"
	"github.com/schuettc/kempt/internal/engine/handlers"
	"github.com/schuettc/kempt/internal/machine"
	"github.com/schuettc/kempt/internal/manifest"
)

// loadSelectedContext reproduces the manifest-read + parse + validate +
// newContext + engine.Select sequence shared by plan, outdated, and upgrade.
func loadSelectedContext(manifestFlag, profileFlag, packagesFlag string, errw io.Writer) (*manifest.Manifest, []*manifest.Package, *machine.Context, error) {
	st, existed, err := loadState()
	if err != nil {
		return nil, nil, nil, err
	}
	manifestPath := resolveManifest(manifestFlag, st, existed)
	profile, packages := resolveSelection(profileFlag, splitPackages(packagesFlag), st, existed)

	src, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, nil, nil, UsageError{Msg: fmt.Sprintf("cannot read %s: %v", manifestPath, err)}
	}
	m, findings := manifest.Parse(src)
	if m != nil {
		findings = append(findings, manifest.Validate(m)...)
	}
	if len(findings) > 0 {
		for _, f := range findings {
			fmt.Fprintf(errw, "%s: %s: %s\n", manifestPath, f.Path, f.Msg)
		}
		return nil, nil, nil, fmt.Errorf("manifest has findings; run kempt lint")
	}
	ctx, err := newContext(filepath.Dir(manifestPath))
	if err != nil {
		return nil, nil, nil, err
	}
	selected, err := engine.Select(m, profile, packages)
	if err != nil {
		return nil, nil, nil, UsageError{Msg: err.Error()}
	}
	return m, selected, ctx, nil
}

// toolStatus is one download tool's version standing.
type toolStatus struct {
	Tool, Bin, Site string
	Mode            string // "pinned" | "latest"
	Installed       string // "" when unknown
	Target          string // pin, or resolved latest
	Behind          bool   // Known && Installed != Target
	Known           bool   // installed version was parseable
}

// scanTools walks every download step in the selected packages and reports its
// version standing. Pinned tools compare offline; "latest" tools resolve
// /dl/<tool>/latest over the network (this is an outdated/upgrade path, which
// is allowed to). A tool whose installed version can't be parsed is Known=false
// and never Behind.
func scanTools(ctx *machine.Context, selected []*manifest.Package) ([]toolStatus, error) {
	var out []toolStatus
	for _, pkg := range selected {
		for _, step := range pkg.Steps {
			st, ok := step.(manifest.DownloadStep)
			if !ok {
				continue
			}
			installed, known := handlers.InstalledToolVersion(ctx, st.Bin)
			ts := toolStatus{Tool: st.Tool, Bin: st.Bin, Site: st.Site, Installed: installed, Known: known}
			if handlers.IsPinnedVersion(st.Version) {
				ts.Mode = "pinned"
				ts.Target = st.Version
			} else {
				ts.Mode = "latest"
				latest, err := handlers.LatestVersion(ctx, st.Site, st.Tool)
				if err != nil {
					return nil, err
				}
				ts.Target = latest
			}
			ts.Behind = known && installed != ts.Target
			out = append(out, ts)
		}
	}
	return out, nil
}
