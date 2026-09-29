# ---- .tools family standard: identical in every family repo ----------------
# `just verify` is exactly what CI runs: the family gate (tools-actions go-ci,
# at the version .github/workflows/ci.yml pins) and then this tool's extras.
# The pre-push hook (lefthook.yml) runs it too, so local and CI never differ.
set shell := ["bash", "-euo", "pipefail", "-c"]

default: verify

verify: gate verify-extra

# The family Go gate: gofmt, vet, golangci-lint (family config), race tests,
# cross-build. Fetched once per tools-actions version into ~/.cache.
gate:
    #!/usr/bin/env bash
    set -euo pipefail
    v="$(grep -oE 'go-ci@v[0-9]+\.[0-9]+\.[0-9]+' .github/workflows/ci.yml | head -1 | cut -d@ -f2)"
    f="${XDG_CACHE_HOME:-$HOME/.cache}/tools-actions/$v/go-ci/local.sh"
    [ -f "$f" ] || { mkdir -p "$(dirname "$f")"; curl -fsSL "https://raw.githubusercontent.com/schuettc/tools-actions/$v/go-ci/local.sh" -o "$f"; }
    bash "$f"

fmt:
    gofmt -w $(git ls-files '*.go')

# ---- kempt ------------------------------------------------------------------
# Tool-specific checks beyond the gate (CI runs this too). kempt has none.
verify-extra:

# A local binary stamped the way a release is.
build:
    go build -ldflags "-X github.com/schuettc/kempt/internal/version.version=$(cat VERSION) -X github.com/schuettc/kempt/internal/version.commit=$(git rev-parse --short HEAD) -X github.com/schuettc/kempt/internal/version.date=$(date -u +%Y-%m-%d)" -o kempt ./cmd/kempt
