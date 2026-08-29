package engine

import (
	"fmt"
	"io"

	"github.com/schuettc/kempt/internal/machine"
	"github.com/schuettc/kempt/internal/manifest"
)

// BuildPlan inspects each selected package against the machine context and
// produces a Plan. A package whose only-clause excludes the context is marked
// Skipped. A step whose only-clause excludes the context yields OpSkip. Steps
// whose kind has no registered handler yield OpBlocked with a placeholder
// detail (handlers are added incrementally during phase 1b); this is not an
// error so plan stays usable mid-phase.
func BuildPlan(ctx *machine.Context, pkgs []*manifest.Package) (*Plan, error) {
	p := &Plan{}
	for _, pkg := range pkgs {
		pp := PackagePlan{Name: pkg.Name}
		if reason, skip := skipReason(ctx, pkg.Only); skip {
			pp.Skipped = true
			pp.Detail = reason
			p.Packages = append(p.Packages, pp)
			continue
		}
		for _, step := range pkg.Steps {
			if reason, skip := skipReason(ctx, stepOnly(step)); skip {
				pp.Steps = append(pp.Steps, StepResult{
					Step:  step,
					Delta: Delta{Op: OpSkip, Detail: fmt.Sprintf("%s (skipped: %s)", step.Kind(), reason)},
				})
				continue
			}
			h, ok := HandlerFor(step.Kind())
			if !ok {
				pp.Steps = append(pp.Steps, StepResult{
					Step:  step,
					Delta: Delta{Op: OpBlocked, Detail: "handler not implemented yet (phase 1b)"},
				})
				continue
			}
			delta, err := h.Inspect(ctx, step)
			if err != nil {
				return nil, fmt.Errorf("inspect %s in %s: %w", step.Kind(), pkg.Name, err)
			}
			pp.Steps = append(pp.Steps, StepResult{Step: step, Delta: delta})
		}
		p.Packages = append(p.Packages, pp)
	}
	return p, nil
}

// Execute applies every OpChange step in plan order. On success it marks the
// StepResult Applied and prints "applied: <detail>". On a step error it records
// the error in the StepResult, prints "! <pkg>: <detail>: <err>", skips the rest
// of that package, and continues with the next package. It returns the number of
// failed steps.
func Execute(ctx *machine.Context, p *Plan, out io.Writer) (failed int) {
	for i := range p.Packages {
		pp := &p.Packages[i]
		if pp.Skipped {
			continue
		}
		for j := range pp.Steps {
			sr := &pp.Steps[j]
			if sr.Delta.Op != OpChange {
				continue
			}
			h, ok := HandlerFor(sr.Step.Kind())
			if !ok {
				continue
			}
			if err := h.Apply(ctx, sr.Step); err != nil {
				sr.Err = err
				fmt.Fprintf(out, "! %s: %s: %v\n", pp.Name, sr.Delta.Detail, err)
				failed++
				break
			}
			sr.Applied = true
			fmt.Fprintf(out, "applied: %s\n", sr.Delta.Detail)
		}
	}
	return failed
}

// FilterByClass returns a copy of p containing only OpChange steps whose
// Step.Class() == class. Packages left with zero steps are dropped, and skipped
// packages are omitted. The original plan and its steps are never mutated. It
// is used by refresh to apply files-class changes without touching software.
func FilterByClass(p *Plan, class manifest.Class) *Plan {
	out := &Plan{}
	for i := range p.Packages {
		pp := &p.Packages[i]
		if pp.Skipped {
			continue
		}
		var steps []StepResult
		for _, sr := range pp.Steps {
			if sr.Delta.Op != OpChange {
				continue
			}
			if sr.Step.Class() != class {
				continue
			}
			steps = append(steps, sr)
		}
		if len(steps) == 0 {
			continue
		}
		out.Packages = append(out.Packages, PackagePlan{
			Name:    pp.Name,
			Skipped: pp.Skipped,
			Detail:  pp.Detail,
			Steps:   steps,
		})
	}
	return out
}

// OnlySkip reports whether an only-clause excludes the machine context.
// It returns a short human-readable reason ("os != windows") and true when
// the step/package should be skipped, or ("", false) when it should run.
// Callers other than BuildPlan (e.g. cli.verify) must use this function
// instead of duplicating the matching logic.
func OnlySkip(ctx *machine.Context, only *manifest.Only) (reason string, skip bool) {
	return skipReason(ctx, only)
}

// StepOnly extracts the only-clause from a step, returning nil when the step
// type has no only-clause or when the type is unknown.
func StepOnly(s manifest.Step) *manifest.Only {
	return stepOnly(s)
}

// skipReason reports whether an only-clause excludes the context, and if so a
// short reason naming the required value that did not match.
func skipReason(ctx *machine.Context, only *manifest.Only) (string, bool) {
	if only == nil {
		return "", false
	}
	if only.OS != "" && only.OS != ctx.OS {
		return "os != " + only.OS, true
	}
	if only.Arch != "" && only.Arch != ctx.Arch {
		return "arch != " + only.Arch, true
	}
	return "", false
}

// stepOnly extracts the only-clause from a step via type switch.
func stepOnly(s manifest.Step) *manifest.Only {
	switch v := s.(type) {
	case manifest.InstallStep:
		return v.Only
	case manifest.GithubReleaseStep:
		return v.Only
	case manifest.GitCloneStep:
		return v.Only
	case manifest.ServiceStep:
		return v.Only
	case manifest.SymlinkStep:
		return v.Only
	case manifest.JSONMergeStep:
		return v.Only
	case manifest.LineInFileStep:
		return v.Only
	case manifest.VerifyStep:
		return v.Only
	}
	return nil
}
