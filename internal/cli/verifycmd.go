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
	"github.com/schuettc/kempt/internal/manifest"
)

func init() {
	Register(Command{Name: "verify", Summary: "run read-only verify checks", Run: runVerify})
}

func runVerify(args []string, out, errw io.Writer) error {
	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
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

	ctx, err := newContext(filepath.Dir(*manifestPath))
	if err != nil {
		return err
	}
	selected, err := engine.Select(m, *profile, packages)
	if err != nil {
		return UsageError{Msg: err.Error()}
	}

	h, ok := engine.HandlerFor("verify")
	if !ok {
		return fmt.Errorf("verify handler not registered")
	}

	var passed, failed, total int
	for _, pkg := range selected {
		for _, step := range pkg.Steps {
			if step.Kind() != "verify" {
				continue
			}
			total++
			delta, err := h.Inspect(ctx, step)
			if err != nil {
				return err
			}
			fmt.Fprintln(out, delta.Detail)
			if delta.Op == engine.OpBlocked {
				failed++
			} else {
				passed++
			}
		}
	}

	if total == 0 {
		fmt.Fprintln(out, "no verify steps")
		return nil
	}
	fmt.Fprintf(out, "%d passed, %d failed\n", passed, failed)
	if failed > 0 {
		return fmt.Errorf("%d verify check(s) failed", failed)
	}
	return nil
}
