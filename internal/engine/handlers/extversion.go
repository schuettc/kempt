package handlers

import (
	"fmt"
	"strings"

	"github.com/schuettc/kempt/internal/inventory"
	"github.com/schuettc/kempt/internal/machine"
	"github.com/schuettc/kempt/internal/manifest"
)

// ExtEntry is one rolling npm/pi extension entry from an install step. Backend
// is "pi" (npm-sourced pi package), "git" (git-sourced pi package) or "npm";
// Entry is the manifest string as pi/npm consume it (for example
// "npm:pi-creel" for pi, "git:github.com/obra/superpowers" for git,
// "typescript" for npm); Pkg is the npm registry package name used to resolve
// latest ("pi-creel", "typescript", "@scope/x"), or for git the repo
// ("github.com/obra/superpowers").
type ExtEntry struct {
	Backend string
	Entry   string
	Pkg     string
}

// RollingExtensions returns the unversioned (rolling) npm and pi entries in
// step. An unversioned git: pi entry is rolling too: it tracks its repo's
// default branch. Pinned entries (name@version) are excluded: they converge offline via
// the install handler and are never rolled to latest.
func RollingExtensions(step manifest.InstallStep) []ExtEntry {
	var out []ExtEntry
	for _, e := range step.Pi {
		name, ver := splitNameVersion(e)
		if ver != "" {
			continue
		}
		// pi entries can be local paths or other sources; only npm-registry
		// (npm:<pkg>) and git (git:<repo>) sources are rolling.
		if strings.HasPrefix(name, "git:") {
			out = append(out, ExtEntry{Backend: "git", Entry: e, Pkg: strings.TrimPrefix(name, "git:")})
			continue
		}
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

// gitShaLen is how many characters of a commit sha git entries report as
// their version: enough to be unique, short enough to read in outdated.
const gitShaLen = 12

// shortSha returns the first field of out truncated to gitShaLen, or "" when
// out is empty.
func shortSha(out string) string {
	f := strings.Fields(out)
	if len(f) == 0 {
		return ""
	}
	if len(f[0]) > gitShaLen {
		return f[0][:gitShaLen]
	}
	return f[0]
}

// gitRemoteURL turns a pi git source (without its git: prefix) into a URL git
// can query: a bare host/owner/repo gets https://, full URLs and scp-style
// git@host:repo are used as-is.
func gitRemoteURL(repo string) string {
	if strings.Contains(repo, "://") || strings.HasPrefix(repo, "git@") {
		return repo
	}
	return "https://" + repo
}

// ExtLatest resolves the newest published version of an extension: `npm view
// <pkg> version` for npm-sourced entries, or the remote default-branch head
// (`git ls-remote <url> HEAD`, short sha) for git entries. Network; memoized
// per run via ctx.Cache. Callers must be an explicit outdated/upgrade/
// update-roll path, never Inspect.
func ExtLatest(ctx *machine.Context, e ExtEntry) (string, error) {
	if e.Backend == "git" {
		out, err := cachedRun(ctx, "git ls-remote "+gitRemoteURL(e.Pkg)+" HEAD")
		if err != nil {
			return "", err
		}
		sha := shortSha(out)
		if sha == "" {
			return "", fmt.Errorf("git ls-remote %s: no HEAD", e.Pkg)
		}
		return sha, nil
	}
	out, err := cachedRun(ctx, npmViewCmd(e.Pkg))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// ExtBehind reports whether installed is behind target. npm-sourced entries
// compare as semver; git entries are behind whenever the shas differ, since
// the target is the default-branch head. An unknown installed version is
// never behind.
func ExtBehind(e ExtEntry, target, installed string) bool {
	if installed == "" {
		return false
	}
	if e.Backend == "git" {
		return target != "" && target != installed
	}
	return SemverNewer(target, installed)
}

// ExtInstalledVersion returns the installed version of a rolling entry and
// whether it was found and parseable. pi entries are read from `pi list` (keyed
// npm:<name>); git entries are the short sha of the clone's HEAD at the path
// `pi list` resolves; npm entries from the global npm inventory.
func ExtInstalledVersion(ctx *machine.Context, e ExtEntry) (version string, known bool) {
	name, _ := splitNameVersion(e.Entry)
	if e.Backend == "git" {
		paths, err := inventory.PiPaths(ctx)
		if err != nil || paths[name] == "" {
			return "", false
		}
		out, err := ctx.Runner.Run("git", "-C", paths[name], "rev-parse", "HEAD")
		if err != nil {
			return "", false
		}
		sha := shortSha(out)
		return sha, sha != ""
	}
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
// <entry>` for pi entries (pi resolves latest for npm:, and for an existing
// git: clone fetches the default branch and resets to it) or `npm install -g
// <pkg>@latest`. It then invalidates the backend's inventory cache key so a
// later read re-probes.
func RollExtension(ctx *machine.Context, e ExtEntry) error {
	if e.Backend == "pi" || e.Backend == "git" {
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
