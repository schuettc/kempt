package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/schuettc/kempt/internal/engine"
	_ "github.com/schuettc/kempt/internal/engine/handlers"
	"github.com/schuettc/kempt/internal/machine"
	"github.com/schuettc/kempt/internal/manifest"
	"github.com/schuettc/kempt/internal/run"
)

func init() {
	Register(Command{Name: "plan", Summary: "show what apply would change", Run: runPlan})
}

func runPlan(args []string, out, errw io.Writer) error {
	fs := flag.NewFlagSet("plan", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	manifestPath := fs.String("manifest", "kempt.toml", "path to manifest")
	profile := fs.String("profile", "", "profile to select")
	packagesFlag := fs.String("packages", "", "comma-separated package names")
	if err := fs.Parse(args); err != nil {
		return UsageError{Msg: err.Error()}
	}

	src, err := os.ReadFile(*manifestPath)
	if err != nil {
		return UsageError{Msg: fmt.Sprintf("cannot read %s: %v", *manifestPath, err)}
	}
	m, findings := manifest.Parse(src)
	if m != nil {
		findings = append(findings, manifest.Validate(m)...)
	}
	if len(findings) > 0 {
		for _, f := range findings {
			fmt.Fprintf(errw, "%s: %s: %s\n", *manifestPath, f.Path, f.Msg)
		}
		return fmt.Errorf("manifest has findings; run kempt lint")
	}

	var packages []string
	if *packagesFlag != "" {
		for _, p := range strings.Split(*packagesFlag, ",") {
			if p = strings.TrimSpace(p); p != "" {
				packages = append(packages, p)
			}
		}
	}

	ctx, err := machine.New(filepath.Dir(*manifestPath), run.RealRunner{})
	if err != nil {
		return err
	}
	selected, err := engine.Select(m, *profile, packages)
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
