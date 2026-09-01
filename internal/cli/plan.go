package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/schuettc/kempt/internal/engine"
	_ "github.com/schuettc/kempt/internal/engine/handlers"
	"github.com/schuettc/kempt/internal/machine"
	"github.com/schuettc/kempt/internal/manifest"
	"github.com/schuettc/kempt/internal/run"
)

func init() {
	Register(Command{Name: "plan", Summary: "show what apply would change", Run: runPlan})
}

// newContext builds a machine.Context for a repo directory. It is a package var
// so tests can inject a Context backed by FakeRunner/FakeReleases.
var newContext = func(repoDir string) (*machine.Context, error) {
	abs, err := filepath.Abs(repoDir)
	if err != nil {
		return nil, err
	}
	return machine.New(abs, run.RealRunner{})
}

func runPlan(args []string, out, errw io.Writer) error {
	fs := flag.NewFlagSet("plan", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	manifestFlag := fs.String("manifest", "", "path to manifest")
	profileFlag := fs.String("profile", "", "profile to select")
	packagesFlag := fs.String("packages", "", "comma-separated package names")
	osFlag := fs.String("os", "", "override OS for dry-planning (e.g. linux, darwin)")
	archFlag := fs.String("arch", "", "override Arch for dry-planning (e.g. amd64, arm64)")
	if err := fs.Parse(args); err != nil {
		return UsageError{Msg: err.Error()}
	}

	st, existed, err := loadState()
	if err != nil {
		return err
	}
	manifestPath := resolveManifest(*manifestFlag, st, existed)
	profile, packages := resolveSelection(*profileFlag, splitPackages(*packagesFlag), st, existed)

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

	ctx, err := newContext(filepath.Dir(manifestPath))
	if err != nil {
		return err
	}
	if *osFlag != "" {
		ctx.OS = *osFlag
	}
	if *archFlag != "" {
		ctx.Arch = *archFlag
	}
	selected, err := engine.Select(m, profile, packages)
	if err != nil {
		return UsageError{Msg: err.Error()}
	}
	plan, err := engine.BuildPlan(ctx, selected)
	if err != nil {
		return err
	}
	engine.Render(plan, out)
	return nil
}
