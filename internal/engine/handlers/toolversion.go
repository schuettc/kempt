package handlers

import (
	"regexp"
	"strings"

	"github.com/schuettc/kempt/internal/machine"
)

// semverRe matches a bare semantic version, optionally with a prerelease
// suffix (e.g. 0.7.0-schuettc.2). No leading v — the family download contract
// keys on bare semver.
var semverRe = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?$`)

// InstalledToolVersion runs `<bin> version` and returns the semver token from
// its output. tools-common binaries print "<tool> <semver> (<commit>, <date>)".
// known is false when the binary is absent, the command errors, or the output
// carries no semver token (e.g. a local "dev" build) — callers then fall back
// to presence-only and never report false drift.
func InstalledToolVersion(ctx *machine.Context, bin string) (version string, known bool) {
	out, err := ctx.Runner.Run(binPath(ctx, bin), "version")
	if err != nil {
		return "", false
	}
	for _, f := range strings.Fields(out) {
		if semverRe.MatchString(f) {
			return f, true
		}
	}
	return "", false
}

// IsPinnedVersion reports whether v names a concrete version to converge to,
// as opposed to the rolling "" / "latest".
func IsPinnedVersion(v string) bool { return v != "" && v != "latest" }

// LatestVersion reads the plain-text /dl/<tool>/latest pointer off the site.
// Network — callers must be an explicit outdated/upgrade path, never Inspect.
func LatestVersion(ctx *machine.Context, site, tool string) (string, error) {
	body, err := ctx.Releases.Download("https://" + site + "/dl/" + tool + "/latest")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(body)), nil
}
