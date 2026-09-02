package cli

import (
	"flag"
	"io"

	"github.com/schuettc/kempt/internal/engine"
	_ "github.com/schuettc/kempt/internal/engine/handlers"
	"github.com/schuettc/kempt/internal/machine"
	"github.com/schuettc/kempt/internal/run"
)

func init() {
	Register(Command{Name: "plan", Summary: "show what apply would change", Run: runPlan})
}

// newContext builds a machine.Context for a repo directory. It is a package var
// so tests can inject a Context backed by FakeRunner/FakeReleases.
var newContext = func(repoDir string) (*machine.Context, error) {
	return machine.New(repoDir, run.RealRunner{})
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

	_, selected, ctx, err := loadSelectedContext(*manifestFlag, *profileFlag, *packagesFlag, errw)
	if err != nil {
		return err
	}
	if *osFlag != "" {
		ctx.OS = *osFlag
	}
	if *archFlag != "" {
		ctx.Arch = *archFlag
	}
	plan, err := engine.BuildPlan(ctx, selected)
	if err != nil {
		return err
	}
	engine.Render(plan, out)
	return nil
}
