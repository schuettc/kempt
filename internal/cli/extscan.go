package cli

import (
	"github.com/schuettc/kempt/internal/engine/handlers"
	"github.com/schuettc/kempt/internal/machine"
	"github.com/schuettc/kempt/internal/manifest"
)

// scanExtensions walks the rolling npm/pi entries in the selected packages and
// reports each entry's version standing as a toolStatus (Kind "pi"/"npm",
// Mode "latest"). Latest is resolved live via npm (an outdated/upgrade/
// update-roll path). A resolution failure is reported on that entry (Err set,
// Behind false) rather than aborting the scan, mirroring scanTools.
func scanExtensions(ctx *machine.Context, selected []*manifest.Package) ([]toolStatus, error) {
	var out []toolStatus
	for _, pkg := range selected {
		for _, step := range pkg.Steps {
			is, ok := step.(manifest.InstallStep)
			if !ok {
				continue
			}
			for _, e := range handlers.RollingExtensions(is) {
				installed, known := handlers.ExtInstalledVersion(ctx, e)
				ts := toolStatus{Kind: e.Backend, Tool: e.Pkg, Ext: e, Mode: "latest", Installed: installed, Known: known}
				latest, err := handlers.ExtLatest(ctx, e.Pkg)
				if err != nil {
					ts.Err = err
				} else {
					ts.Target = latest
					ts.Behind = known && handlers.SemverNewer(ts.Target, installed)
				}
				out = append(out, ts)
			}
		}
	}
	return out, nil
}
