package handlers

import (
	"fmt"
	"sort"
	"strings"

	"github.com/schuettc/kempt/internal/engine"
	"github.com/schuettc/kempt/internal/machine"
	"github.com/schuettc/kempt/internal/manifest"
)

func init() { engine.RegisterHandler(installHandler{}) }

// installHandler realises software installation for an install step. Backend
// selection happens at inspect time:
//
//   - darwin/linux with brew content → brew backend.
//   - matching-OS winget (windows) or apt (linux) content but no brew →
//     OpBlocked "(backend <name> not implemented in this phase)".
//   - no backend applies to this OS → OpSkip "(no backend for <os>)".
//
// First-match wins: on linux with BOTH brew and apt content the brew backend is
// selected and apt is ignored.
type installHandler struct{}

// brew inventory command keys, memoized via ctx.Cache.
const (
	brewFormulaCmd = "brew list --formula -1"
	brewCaskCmd    = "brew list --cask -1"
	brewTapCmd     = "brew tap"
)

func (installHandler) Kind() string { return "install" }

func (installHandler) Inspect(ctx *machine.Context, s manifest.Step) (engine.Delta, error) {
	st := s.(manifest.InstallStep)

	if useBrew(ctx, st) {
		return brewInspect(ctx, st.Brew)
	}
	// A matching-OS non-brew backend is present but unimplemented this phase.
	if name, ok := unimplementedBackend(ctx, st); ok {
		return engine.Delta{
			Op:     engine.OpBlocked,
			Detail: fmt.Sprintf("install (backend %s not implemented in this phase)", name),
		}, nil
	}
	return engine.Delta{
		Op:     engine.OpSkip,
		Detail: fmt.Sprintf("install (no backend for %s)", ctx.OS),
	}, nil
}

func (installHandler) Apply(ctx *machine.Context, s manifest.Step) error {
	st := s.(manifest.InstallStep)
	if !useBrew(ctx, st) {
		// Non-brew backends are inspect-only in this phase; Apply is only
		// reached for OpChange, which only the brew backend produces.
		return nil
	}
	return brewApply(ctx, st.Brew)
}

// useBrew reports whether the brew backend applies: darwin/linux with brew
// content.
func useBrew(ctx *machine.Context, st manifest.InstallStep) bool {
	if ctx.OS != "darwin" && ctx.OS != "linux" {
		return false
	}
	return st.Brew != nil && hasBrewContent(st.Brew)
}

func hasBrewContent(b *manifest.BrewSpec) bool {
	return len(b.Formulas) > 0 || len(b.Casks) > 0 || len(b.Taps) > 0
}

// unimplementedBackend names a matching-OS non-brew backend that has content
// but is not implemented in this phase.
func unimplementedBackend(ctx *machine.Context, st manifest.InstallStep) (string, bool) {
	switch ctx.OS {
	case "windows":
		if len(st.Winget) > 0 {
			return "winget", true
		}
	case "linux":
		if len(st.Apt) > 0 {
			return "apt", true
		}
	}
	return "", false
}

// brewInspect probes brew inventory (read-only) and reports the delta. It runs
// only the three inventory commands, memoized via ctx.Cache.
func brewInspect(ctx *machine.Context, spec *manifest.BrewSpec) (engine.Delta, error) {
	if _, err := ctx.Runner.LookPath("brew"); err != nil {
		return engine.Delta{Op: engine.OpBlocked, Detail: "install (brew not found)"}, nil
	}

	installedF, err := brewInventory(ctx, brewFormulaCmd)
	if err != nil {
		return engine.Delta{}, err
	}
	installedC, err := brewInventory(ctx, brewCaskCmd)
	if err != nil {
		return engine.Delta{}, err
	}
	installedT, err := brewInventory(ctx, brewTapCmd)
	if err != nil {
		return engine.Delta{}, err
	}

	missF := missing(spec.Formulas, installedF)
	missC := missing(spec.Casks, installedC)
	missT := missing(spec.Taps, installedT)

	if len(missF) == 0 && len(missC) == 0 && len(missT) == 0 {
		total := len(spec.Formulas) + len(spec.Casks) + len(spec.Taps)
		return engine.Delta{Op: engine.OpNoop, Detail: fmt.Sprintf("brew: %d present", total)}, nil
	}

	// Build labeled segments; formulas carry no label, casks/taps do. Join
	// only non-empty segments so casks-only yields "brew install: casks: X"
	// rather than "brew install: ; casks: X".
	var segments []string
	if len(missF) > 0 {
		segments = append(segments, strings.Join(missF, " "))
	}
	if len(missC) > 0 {
		segments = append(segments, "casks: "+strings.Join(missC, " "))
	}
	if len(missT) > 0 {
		segments = append(segments, "taps: "+strings.Join(missT, " "))
	}
	return engine.Delta{Op: engine.OpChange, Detail: "brew install: " + strings.Join(segments, "; ")}, nil
}

// brewApply installs the missing taps, formulas, and casks in that order,
// coalescing formulas and casks into a single command each, then invalidates
// the inventory cache keys.
func brewApply(ctx *machine.Context, spec *manifest.BrewSpec) error {
	installedF, err := brewInventory(ctx, brewFormulaCmd)
	if err != nil {
		return err
	}
	installedC, err := brewInventory(ctx, brewCaskCmd)
	if err != nil {
		return err
	}
	installedT, err := brewInventory(ctx, brewTapCmd)
	if err != nil {
		return err
	}

	missF := missing(spec.Formulas, installedF)
	missC := missing(spec.Casks, installedC)
	missT := missing(spec.Taps, installedT)

	for _, t := range missT {
		if _, err := ctx.Runner.Run("brew", "tap", t); err != nil {
			return err
		}
	}
	if len(missF) > 0 {
		if _, err := ctx.Runner.Run("brew", append([]string{"install"}, missF...)...); err != nil {
			return err
		}
	}
	if len(missC) > 0 {
		if _, err := ctx.Runner.Run("brew", append([]string{"install", "--cask"}, missC...)...); err != nil {
			return err
		}
	}

	// Inventory changed; drop memoized results so any later probe re-reads.
	delete(ctx.Cache, brewFormulaCmd)
	delete(ctx.Cache, brewCaskCmd)
	delete(ctx.Cache, brewTapCmd)
	return nil
}

// brewInventory runs a read-only inventory command once per run, memoizing the
// stdout on ctx.Cache keyed by the FakeRunner key string.
func brewInventory(ctx *machine.Context, key string) (map[string]struct{}, error) {
	out, ok := ctx.Cache[key]
	if !ok {
		fields := strings.Fields(key)
		var err error
		out, err = ctx.Runner.Run(fields[0], fields[1:]...)
		if err != nil {
			return nil, err
		}
		if ctx.Cache != nil {
			ctx.Cache[key] = out
		}
	}
	return toSet(out), nil
}

// toSet splits command output into a set of non-empty trimmed lines.
func toSet(out string) map[string]struct{} {
	set := map[string]struct{}{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			set[line] = struct{}{}
		}
	}
	return set
}

// missing returns the desired items absent from installed, sorted.
func missing(desired []string, installed map[string]struct{}) []string {
	var out []string
	for _, d := range desired {
		if _, ok := installed[d]; !ok {
			out = append(out, d)
		}
	}
	sort.Strings(out)
	return out
}
