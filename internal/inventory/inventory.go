// Package inventory reads the set of globally-installed pi and npm packages as
// name→version maps, memoized via a machine.Context cache. It is shared by the
// install handler (to compute install deltas) and by higher-level commands
// (e.g. doctor) that report what is currently present.
package inventory

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/schuettc/kempt/internal/machine"
)

// Inventory command keys. These strings double as ctx.Cache memoization keys
// (FakeRunner key format), so callers that invalidate the cache must delete
// these exact keys.
const (
	// NpmCmd lists globally-installed packages as JSON. The `.dependencies`
	// object maps package NAME (scoped names like "@scope/name" are keys) to an
	// object carrying `.version`, letting us build name→version.
	NpmCmd = "npm ls -g --depth=0 --json"
	// PiCmd lists registered package identifiers, one per line.
	PiCmd = "pi list"
)

// Npm returns globally-installed npm packages as name→version, memoized via
// ctx.Cache. It parses `npm ls -g --depth=0 --json`, reading the top-level
// `.dependencies` object (keys are package names, preserving @scope/name). A
// parse failure is treated as an error (not an empty inventory).
func Npm(ctx *machine.Context) (map[string]string, error) {
	out, err := cachedRun(ctx, NpmCmd)
	if err != nil {
		return nil, err
	}
	inv := map[string]string{}
	var parsed struct {
		Dependencies map[string]struct {
			Version string `json:"version"`
		} `json:"dependencies"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		return nil, fmt.Errorf("npm inventory: unparseable output: %w", err)
	}
	for name, dep := range parsed.Dependencies {
		if name != "" {
			inv[name] = dep.Version
		}
	}
	return inv, nil
}

// Pi returns registered pi packages as name→version, memoized via ctx.Cache.
// For `npm:<name>@<version>` lines the key is `npm:<name>` (prefix and any
// @scope preserved) and the value is `<version>`; local-path and other lines
// are kept verbatim as the key with an empty (unversioned) value. Section
// headers (e.g. "User packages:") and blank lines are skipped.
func Pi(ctx *machine.Context) (map[string]string, error) {
	out, err := cachedRun(ctx, PiCmd)
	if err != nil {
		return nil, err
	}
	inv := map[string]string{}
	// Real `pi list` output is two lines per package: a spec line, then a
	// deeper-indented resolved path. Track the indent of the first content
	// (non-blank, non-header) line as the package-entry indent; any content
	// line indented MORE than that is a resolved-path continuation and is
	// skipped. Measure indentation on the raw line, before trimming.
	entryIndent := -1
	for _, raw := range strings.Split(out, "\n") {
		line := strings.TrimSpace(raw)
		// Skip blank lines and section-header lines (e.g. "User packages:").
		if line == "" || strings.HasSuffix(line, ":") {
			continue
		}
		indent := len(raw) - len(strings.TrimLeft(raw, " \t"))
		if entryIndent == -1 {
			entryIndent = indent
		}
		if indent > entryIndent {
			// Deeper-indented resolved-path continuation line.
			continue
		}
		name, ver := splitNameVersion(line)
		inv[name] = ver
	}
	return inv, nil
}

// splitNameVersion splits an entry into (name, version) at a trailing @version,
// preserving an `npm:` prefix and never treating a leading @scope as a version
// separator.
//
// Examples:
//
//	"npm:pi-tmux-bridge@0.1.1"            → ("npm:pi-tmux-bridge", "0.1.1")
//	"npm:pi-tmux-bridge"                  → ("npm:pi-tmux-bridge", "")
//	"@earendil-works/pi-coding-agent"     → ("@earendil-works/pi-coding-agent", "")
//	"@earendil-works/pi-coding-agent@1.2.3" → ("@earendil-works/pi-coding-agent", "1.2.3")
//	"/local/path"                         → ("/local/path", "")
func splitNameVersion(entry string) (name, version string) {
	const prefix = "npm:"
	work := entry
	pre := ""
	if strings.HasPrefix(entry, prefix) {
		pre = prefix
		work = entry[len(prefix):]
	}
	// at > 0 guard so a leading "@scope" is not treated as a version separator.
	if at := strings.LastIndex(work, "@"); at > 0 {
		return pre + work[:at], work[at+1:]
	}
	return entry, ""
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
