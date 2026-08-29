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

// installHandler realises software installation for an install step.
//
// Backends fall into two categories:
//
//   - OS-EXCLUSIVE software backend (brew/winget/apt): exactly one applies per
//     OS, first-match wins. darwin/linux with brew content → brew; matching-OS
//     winget (windows) or apt (linux) with no brew → OpBlocked "(backend <name>
//     not implemented in this phase)"; nothing matched → no OS backend. On
//     linux with BOTH brew and apt content, brew is selected and apt ignored.
//   - ADDITIVE cross-platform sources (npm/pi): each applies independently on
//     any OS whenever its list is non-empty. A single install step may carry
//     brew AND npm AND pi, and every present source is evaluated and applied.
//
// Inspect evaluates every present backend, joins their Detail segments with
// "; ", and combines their ops with this precedence:
//
//  1. OpChange if ANY backend reports changes (Apply must run);
//  2. else OpBlocked if ANY required backend tool is missing (that backend
//     cannot converge, so the step is blocked);
//  3. else OpNoop.
//
// If no backend applies at all, the step is OpSkip "(no backend for <os>)".
type installHandler struct{}

// inventory command keys, memoized via ctx.Cache.
const (
	brewFormulaCmd = "brew list --formula -1"
	brewCaskCmd    = "brew list --cask -1"
	brewTapCmd     = "brew tap"
	// npmInventoryCmd lists globally-installed packages. --parseable prints one
	// absolute install path per package; the package name is the path segment
	// after "node_modules/" (which preserves @scope/name for scoped packages).
	npmInventoryCmd = "npm ls -g --depth=0 --parseable"
	// piInventoryCmd lists registered package identifiers, one per line.
	piInventoryCmd = "pi list"
)

func (installHandler) Kind() string { return "install" }

func (installHandler) Inspect(ctx *machine.Context, s manifest.Step) (engine.Delta, error) {
	st := s.(manifest.InstallStep)

	var subs []engine.Delta

	// OS-exclusive software backend (one wins per OS).
	if useBrew(ctx, st) {
		d, err := brewInspect(ctx, st.Brew)
		if err != nil {
			return engine.Delta{}, err
		}
		subs = append(subs, d)
	} else if name, ok := unimplementedBackend(ctx, st); ok {
		subs = append(subs, engine.Delta{
			Op:     engine.OpBlocked,
			Detail: fmt.Sprintf("install (backend %s not implemented in this phase)", name),
		})
	}

	// Additive cross-platform sources.
	if len(st.Npm) > 0 {
		d, err := npmInspect(ctx, st.Npm)
		if err != nil {
			return engine.Delta{}, err
		}
		subs = append(subs, d)
	}
	if len(st.Pi) > 0 {
		d, err := piInspect(ctx, st.Pi)
		if err != nil {
			return engine.Delta{}, err
		}
		subs = append(subs, d)
	}

	if len(subs) == 0 {
		return engine.Delta{
			Op:     engine.OpSkip,
			Detail: fmt.Sprintf("install (no backend for %s)", ctx.OS),
		}, nil
	}
	return combineDeltas(subs), nil
}

// combineDeltas joins sub-backend deltas: Detail joins with "; " in evaluation
// order; Op = OpChange if any changes, else OpBlocked if any is blocked, else
// OpNoop.
func combineDeltas(subs []engine.Delta) engine.Delta {
	details := make([]string, 0, len(subs))
	op := engine.OpNoop
	blocked := false
	for _, d := range subs {
		details = append(details, d.Detail)
		switch d.Op {
		case engine.OpChange:
			op = engine.OpChange
		case engine.OpBlocked:
			blocked = true
		}
	}
	if op != engine.OpChange && blocked {
		op = engine.OpBlocked
	}
	return engine.Delta{Op: op, Detail: strings.Join(details, "; ")}
}

func (installHandler) Apply(ctx *machine.Context, s manifest.Step) error {
	st := s.(manifest.InstallStep)
	// Run every actionable backend in evaluation order. Each applier recomputes
	// its own missing set and no-ops when nothing is due.
	if useBrew(ctx, st) {
		if err := brewApply(ctx, st.Brew); err != nil {
			return err
		}
	}
	if len(st.Npm) > 0 {
		if err := npmApply(ctx, st.Npm); err != nil {
			return err
		}
	}
	if len(st.Pi) > 0 {
		if err := piApply(ctx, st.Pi); err != nil {
			return err
		}
	}
	return nil
}

// npmInspect probes global npm packages (read-only) and reports the delta.
func npmInspect(ctx *machine.Context, desired []string) (engine.Delta, error) {
	if _, err := ctx.Runner.LookPath("npm"); err != nil {
		return engine.Delta{Op: engine.OpBlocked, Detail: "install (npm not found)"}, nil
	}
	installed, err := npmInventory(ctx)
	if err != nil {
		return engine.Delta{}, err
	}
	miss := missing(desired, installed)
	if len(miss) == 0 {
		return engine.Delta{Op: engine.OpNoop, Detail: fmt.Sprintf("npm: %d present", len(desired))}, nil
	}
	return engine.Delta{Op: engine.OpChange, Detail: "npm install: " + strings.Join(miss, " ")}, nil
}

// npmApply installs the missing global packages in one command, then
// invalidates the npm inventory cache key.
func npmApply(ctx *machine.Context, desired []string) error {
	installed, err := npmInventory(ctx)
	if err != nil {
		return err
	}
	miss := missing(desired, installed)
	if len(miss) == 0 {
		return nil
	}
	if _, err := ctx.Runner.Run("npm", append([]string{"install", "-g"}, miss...)...); err != nil {
		return err
	}
	delete(ctx.Cache, npmInventoryCmd)
	return nil
}

// npmInventory returns the set of globally-installed npm package names,
// memoized via ctx.Cache.
func npmInventory(ctx *machine.Context) (map[string]struct{}, error) {
	out, err := cachedRun(ctx, npmInventoryCmd)
	if err != nil {
		return nil, err
	}
	set := map[string]struct{}{}
	const marker = "node_modules/"
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		i := strings.LastIndex(line, marker)
		if i < 0 {
			continue // the prefix directory line has no node_modules segment
		}
		if name := line[i+len(marker):]; name != "" {
			set[name] = struct{}{}
		}
	}
	return set, nil
}

// piInspect probes registered pi packages (read-only) and reports the delta.
func piInspect(ctx *machine.Context, desired []string) (engine.Delta, error) {
	if _, err := ctx.Runner.LookPath("pi"); err != nil {
		return engine.Delta{Op: engine.OpBlocked, Detail: "install (pi not found)"}, nil
	}
	present, err := piInventory(ctx)
	if err != nil {
		return engine.Delta{}, err
	}
	miss := missing(desired, present)
	if len(miss) == 0 {
		return engine.Delta{Op: engine.OpNoop, Detail: fmt.Sprintf("pi: %d present", len(desired))}, nil
	}
	return engine.Delta{Op: engine.OpChange, Detail: "pi install: " + strings.Join(miss, " ")}, nil
}

// piApply installs each missing pi package with its own command, then
// invalidates the pi inventory cache key.
func piApply(ctx *machine.Context, desired []string) error {
	present, err := piInventory(ctx)
	if err != nil {
		return err
	}
	miss := missing(desired, present)
	if len(miss) == 0 {
		return nil
	}
	for _, p := range miss {
		if _, err := ctx.Runner.Run("pi", "install", p); err != nil {
			return err
		}
	}
	delete(ctx.Cache, piInventoryCmd)
	return nil
}

// piInventory returns the set of registered pi package identifiers, memoized
// via ctx.Cache. `npm:<name>@<version>` entries are reduced to `npm:<name>`
// (version stripped) so they match unversioned desired identifiers;
// local-path and other entries are kept verbatim.
func piInventory(ctx *machine.Context) (map[string]struct{}, error) {
	out, err := cachedRun(ctx, piInventoryCmd)
	if err != nil {
		return nil, err
	}
	set := map[string]struct{}{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		set[reducePiEntry(line)] = struct{}{}
	}
	return set, nil
}

// reducePiEntry strips a trailing @version from an npm: identifier, preserving
// any @scope prefix. Non-npm entries (e.g. local paths) are returned verbatim.
func reducePiEntry(s string) string {
	const prefix = "npm:"
	if !strings.HasPrefix(s, prefix) {
		return s
	}
	rest := s[len(prefix):]
	if at := strings.LastIndex(rest, "@"); at > 0 {
		rest = rest[:at]
	}
	return prefix + rest
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
	out, err := cachedRun(ctx, key)
	if err != nil {
		return nil, err
	}
	return toSet(out), nil
}

// cachedRun runs the command encoded in key (FakeRunner key format) once per
// run, memoizing stdout on ctx.Cache.
func cachedRun(ctx *machine.Context, key string) (string, error) {
	if out, ok := ctx.Cache[key]; ok {
		return out, nil
	}
	fields := strings.Fields(key)
	out, err := ctx.Runner.Run(fields[0], fields[1:]...)
	if err != nil {
		return "", err
	}
	if ctx.Cache != nil {
		ctx.Cache[key] = out
	}
	return out, nil
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
