package cli

import (
	"github.com/schuettc/kempt/internal/engine/handlers"
	"github.com/schuettc/kempt/internal/machine"
	"github.com/schuettc/kempt/internal/manifest"
)

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
