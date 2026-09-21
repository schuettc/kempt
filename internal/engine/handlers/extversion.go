package handlers

import (
	"strings"

	"github.com/schuettc/kempt/internal/inventory"
	"github.com/schuettc/kempt/internal/machine"
	"github.com/schuettc/kempt/internal/manifest"
)

// ExtEntry is one rolling npm/pi extension entry from an install step. Backend
// is "pi" or "npm"; Entry is the manifest string as pi/npm consume it (for
// example "npm:pi-creel" for pi, "typescript" for npm); Pkg is the npm registry
// package name used to resolve latest ("pi-creel", "typescript", "@scope/x").
type ExtEntry struct {
	Backend string
	Entry   string
	Pkg     string
}

// RollingExtensions returns the unversioned (rolling) npm and pi entries in
// step. Pinned entries (name@version) are excluded: they converge offline via
// the install handler and are never rolled to latest.
func RollingExtensions(step manifest.InstallStep) []ExtEntry {
	var out []ExtEntry
	for _, e := range step.Pi {
		name, ver := splitNameVersion(e)
		if ver != "" {
			continue
		}
		// pi entries can be local paths or non-npm sources; only npm-registry
		// sources (npm:<pkg>) are rolling.
		if !strings.HasPrefix(name, "npm:") {
			continue
		}
		out = append(out, ExtEntry{Backend: "pi", Entry: e, Pkg: registryPkg(name)})
	}
	for _, e := range step.Npm {
		name, ver := splitNameVersion(e)
		if ver != "" {
			continue
		}
		// npm entries can be local paths; skip anything that looks like a path
		// (contains '/' and is not a @scope/name).
		if strings.Contains(name, "/") && !strings.HasPrefix(name, "@") {
			continue
		}
		out = append(out, ExtEntry{Backend: "npm", Entry: e, Pkg: registryPkg(name)})
	}
	return out
}

// registryPkg strips the "npm:" pi-source prefix so the remainder is the npm
// registry package name; a leading @scope is left intact.
func registryPkg(name string) string { return strings.TrimPrefix(name, "npm:") }

// npmViewCmd is the memoized inventory key for `npm view <pkg> version`.
func npmViewCmd(pkg string) string { return "npm view " + pkg + " version" }

// ExtLatest resolves the newest published version of an extension via
// `npm view <pkg> version`. Network; memoized per run via ctx.Cache. Callers
// must be an explicit outdated/upgrade/update-roll path, never Inspect.
func ExtLatest(ctx *machine.Context, pkg string) (string, error) {
	out, err := cachedRun(ctx, npmViewCmd(pkg))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// ExtInstalledVersion returns the installed version of a rolling entry and
// whether it was found and parseable. pi entries are read from `pi list` (keyed
// npm:<name>); npm entries from the global npm inventory.
func ExtInstalledVersion(ctx *machine.Context, e ExtEntry) (version string, known bool) {
	name, _ := splitNameVersion(e.Entry)
	inv, err := extInventory(ctx, e.Backend)
	if err != nil {
		return "", false
	}
	v, ok := inv[name]
	return v, ok && v != ""
}

// extInventory returns the installed-version map for a backend.
func extInventory(ctx *machine.Context, backend string) (map[string]string, error) {
	if backend == "pi" {
		return inventory.Pi(ctx)
	}
	return inventory.Npm(ctx)
}

// RollExtension reinstalls a rolling entry at the newest version: `pi install
// npm:<name>` (pi resolves latest) or `npm install -g <pkg>@latest`. It then
// invalidates the backend's inventory cache key so a later read re-probes.
func RollExtension(ctx *machine.Context, e ExtEntry) error {
	if e.Backend == "pi" {
		if _, err := ctx.Runner.Run("pi", "install", e.Entry); err != nil {
			return err
		}
		delete(ctx.Cache, inventory.PiCmd)
		return nil
	}
	if _, err := ctx.Runner.Run("npm", "install", "-g", e.Pkg+"@latest"); err != nil {
		return err
	}
	delete(ctx.Cache, inventory.NpmCmd)
	return nil
}
