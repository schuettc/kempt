package handlers

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/schuettc/kempt/internal/engine"
	"github.com/schuettc/kempt/internal/machine"
	"github.com/schuettc/kempt/internal/manifest"
)

func init() { engine.RegisterHandler(verifyStepHandler{}) }

// verifyStepHandler evaluates read-only checks. Inspect returns OpNoop when all
// checks pass and OpBlocked when any fail; it never returns OpChange, so the
// engine's Execute (which walks OpChange only) never applies a verify step.
// verify degrades rather than hard-erroring: resolver and runner failures
// become OpBlocked details, keeping `kempt verify` usable offline.
type verifyStepHandler struct{}

func (verifyStepHandler) Kind() string { return "verify" }

func (verifyStepHandler) Inspect(ctx *machine.Context, s manifest.Step) (engine.Delta, error) {
	st := s.(manifest.VerifyStep)

	var details []string
	blocked := false
	fail := func(detail string) { details = append(details, detail); blocked = true }
	pass := func(detail string) { details = append(details, detail) }

	if st.CommandExists != "" {
		if _, err := ctx.Runner.LookPath(st.CommandExists); err == nil {
			pass(fmt.Sprintf("verify: command %s ✓", st.CommandExists))
		} else {
			fail(fmt.Sprintf("verify: command %s MISSING", st.CommandExists))
		}
	}

	if st.SymlinkTarget != nil {
		link := ctx.Expand(st.SymlinkTarget.Link)
		target := ctx.Expand(st.SymlinkTarget.Target)
		cur, err := os.Readlink(link)
		if err == nil && cur == target {
			pass(fmt.Sprintf("verify: symlink %s -> %s ✓", link, target))
		} else {
			got := cur
			if err != nil {
				got = err.Error()
			}
			fail(fmt.Sprintf("verify: symlink %s -> %s MISMATCH (got %s)", link, target, got))
		}
	}

	if st.VersionCurrent != nil {
		vc := st.VersionCurrent
		tag, err := ctx.Releases.LatestTag(vc.Repo)
		if err != nil {
			fail(fmt.Sprintf("verify: %s (cannot resolve latest: %v)", vc.Repo, err))
		} else {
			fields := strings.Fields(vc.Command)
			var out string
			var runErr error
			if len(fields) == 0 {
				runErr = errors.New("empty version command")
			} else {
				out, runErr = ctx.Runner.Run(fields[0], fields[1:]...)
			}
			switch {
			case runErr != nil:
				fail(fmt.Sprintf("verify: %s (version command failed: %v)", vc.Repo, runErr))
			case strings.Contains(out, strings.TrimPrefix(tag, "v")):
				pass(fmt.Sprintf("verify: %s current (%s)", vc.Repo, tag))
			default:
				fail(fmt.Sprintf("verify: %s behind (latest %s)", vc.Repo, tag))
			}
		}
	}

	op := engine.OpNoop
	if blocked {
		op = engine.OpBlocked
	}
	return engine.Delta{Op: op, Detail: strings.Join(details, "; ")}, nil
}

func (verifyStepHandler) Apply(ctx *machine.Context, s manifest.Step) error {
	return errors.New("verify steps are never applied")
}
