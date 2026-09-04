package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/schuettc/kempt/internal/engine"
	_ "github.com/schuettc/kempt/internal/engine/handlers"
	"github.com/schuettc/kempt/internal/gitrepo"
	"github.com/schuettc/kempt/internal/manifest"
	"github.com/schuettc/kempt/internal/state"
)

func init() {
	Register(Command{
		Name:    "refresh",
		Summary: "check the repo and update the cached status",
		Run:     runRefresh,
	})
}

// now is a seam so tests can pin the CheckedAt timestamp.
var now = time.Now

func runRefresh(args []string, out, errw io.Writer) error {
	fs := flag.NewFlagSet("refresh", flag.ContinueOnError)
	manifestFlag := fs.String("manifest", "", "path to manifest")
	if err := ParseFlags(fs, args, out); err != nil {
		return err
	}

	st, existed, err := loadState()
	if err != nil {
		return err
	}
	if !existed {
		return UsageError{Msg: "no saved selection; run kempt init first"}
	}

	manifestPath := resolveManifest(*manifestFlag, st, existed)
	ctx, err := newContext(filepath.Dir(manifestPath))
	if err != nil {
		return err
	}

	// Fetch is best-effort: a failure must not blank the cache. Plan against the
	// current checkout and note the degradation in the summary.
	degraded := false
	if err := gitrepo.Fetch(ctx.Runner, st.RepoDir); err != nil {
		degraded = true
	}
	behind, _ := gitrepo.Behind(ctx.Runner, st.RepoDir)

	src, err := os.ReadFile(manifestPath)
	if err != nil {
		return UsageError{Msg: fmt.Sprintf("cannot read %s: %v", manifestPath, err)}
	}
	m, findings := manifest.Parse(src)
	if m != nil {
		findings = append(findings, manifest.Validate(m)...)
	}
	if len(findings) > 0 {
		for _, f := range findings {
			fmt.Fprintf(errw, "%s: %s: %s\n", manifestPath, f.Path, f.Msg)
		}
		return fmt.Errorf("manifest has findings; run kempt lint")
	}

	selected, err := engine.Select(m, "", st.Packages)
	if err != nil {
		return UsageError{Msg: err.Error()}
	}
	plan, err := engine.BuildPlan(ctx, selected)
	if err != nil {
		return err
	}

	fileChanges, softwareChanges, blocked := countByClass(plan)

	// Split policy: with auto-apply-files enabled we apply ONLY files-class
	// changes, never software. After applying, rebuild the plan so the cached
	// counts reflect post-apply reality (files should now be 0, software
	// unchanged and still pending).
	if st.AutoApplyFiles {
		engine.Execute(ctx, engine.FilterByClass(plan, manifest.ClassFiles), out)
		plan, err = engine.BuildPlan(ctx, selected)
		if err != nil {
			return err
		}
		fileChanges, softwareChanges, blocked = countByClass(plan)
	}

	status := &state.Status{
		CheckedAt:       now(),
		Behind:          behind,
		FileChanges:     fileChanges,
		SoftwareChanges: softwareChanges,
		Blocked:         blocked,
	}
	store, err := statusStore()
	if err != nil {
		return err
	}
	if err := store.SaveStatus(status); err != nil {
		return err
	}

	line := formatStatus(status)
	if degraded {
		line += " (fetch failed; using local checkout)"
	}
	fmt.Fprintln(out, line)
	return nil
}

// countByClass tallies OpChange steps by class (files vs software) and OpBlocked
// steps across a plan, skipping filtered-out packages.
func countByClass(p *engine.Plan) (files, software, blocked int) {
	for _, pp := range p.Packages {
		if pp.Skipped {
			continue
		}
		for _, sr := range pp.Steps {
			switch sr.Delta.Op {
			case engine.OpChange:
				switch sr.Step.Class() {
				case manifest.ClassFiles:
					files++
				case manifest.ClassSoftware:
					software++
				}
			case engine.OpBlocked:
				blocked++
			}
		}
	}
	return files, software, blocked
}
