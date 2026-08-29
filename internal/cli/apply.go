package cli

import (
	"bufio"
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
	Register(Command{Name: "apply", Summary: "apply the plan to converge the machine", Run: runApply})
}

// stdin is the reader used for the apply confirmation prompt. It is a package
// var so tests can inject a scripted response.
var stdin io.Reader = os.Stdin

func runApply(args []string, out, errw io.Writer) error {
	fs := flag.NewFlagSet("apply", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	manifestPath := fs.String("manifest", "kempt.toml", "path to manifest")
	profile := fs.String("profile", "", "profile to select")
	packagesFlag := fs.String("packages", "", "comma-separated package names")
	yes := fs.Bool("yes", false, "skip the confirmation prompt")
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
	plan, err := engine.BuildPlan(ctx, selected)
	if err != nil {
		return err
	}

	// Show the same plan `kempt plan` would.
	engine.Render(plan, out)

	changes := countChanges(plan)
	if changes == 0 {
		fmt.Fprintln(out, "nothing to do")
		return nil
	}

	if !*yes {
		fmt.Fprintf(out, "apply %d changes? [y/N] ", changes)
		line, _ := bufio.NewReader(stdin).ReadString('\n')
		switch strings.ToLower(strings.TrimSpace(line)) {
		case "y", "yes":
			// proceed
		default:
			fmt.Fprintln(out, "aborted")
			return fmt.Errorf("aborted")
		}
	}

	failed := engine.Execute(ctx, plan, out)

	// Honesty check: re-inspect every step Execute claims it applied. If any is
	// not now OpNoop, the machine did not actually converge — count it failed.
	applied := 0
	for i := range plan.Packages {
		pp := &plan.Packages[i]
		for j := range pp.Steps {
			sr := &pp.Steps[j]
			if !sr.Applied {
				continue
			}
			applied++
			h, ok := engine.HandlerFor(sr.Step.Kind())
			if !ok {
				continue
			}
			delta, err := h.Inspect(ctx, sr.Step)
			if err != nil {
				fmt.Fprintf(out, "! %s: re-inspect failed: %v\n", pp.Name, err)
				failed++
				continue
			}
			if delta.Op != engine.OpNoop {
				fmt.Fprintf(out, "! %s: not converged after apply: %s\n", pp.Name, delta.Detail)
				failed++
			}
		}
	}

	blocked := countBlocked(plan)
	fmt.Fprintf(out, "%d applied, %d failed\n", applied, failed)
	if blocked > 0 {
		fmt.Fprintf(out, "%d blocked (unresolved)\n", blocked)
	}

	if failed > 0 {
		return fmt.Errorf("%d step(s) failed", failed)
	}
	return nil
}

func countChanges(p *engine.Plan) int {
	n := 0
	for _, pp := range p.Packages {
		if pp.Skipped {
			continue
		}
		for _, sr := range pp.Steps {
			if sr.Delta.Op == engine.OpChange {
				n++
			}
		}
	}
	return n
}

func countBlocked(p *engine.Plan) int {
	n := 0
	for _, pp := range p.Packages {
		if pp.Skipped {
			continue
		}
		for _, sr := range pp.Steps {
			if sr.Delta.Op == engine.OpBlocked {
				n++
			}
		}
	}
	return n
}
