#!/bin/sh
# kempt installer — downloads the latest release binary for this machine,
# verifies its checksum, and installs it to ~/.local/bin (override with
# KEMPT_INSTALL_DIR). Usage:
#
#   curl -fsSL https://kempt.tools/install.sh | sh
#
# kempt is a single, self-contained binary; there is nothing else to install.
#
# NOTE: kempt's release binaries are UNSIGNED. On macOS, Gatekeeper only
# quarantines files it recognizes as downloaded through a browser or a
# quarantine-aware app; a binary fetched by curl and written to disk by this
# script is not quarantined, so it runs without a "cannot be opened" prompt.
# Installing via the curl | sh one-liner above is therefore the supported path.
set -eu

repo="schuettc/kempt"
install_dir="${KEMPT_INSTALL_DIR:-$HOME/.local/bin}"
base="https://github.com/$repo/releases/latest/download"

fail() { printf 'kempt install: %s\n' "$*" >&2; exit 1; }

for arg in "$@"; do
  case "$arg" in
    -h|--help)
      printf 'usage: install.sh\n'
      printf '  Installs the latest kempt release to %s\n' "${KEMPT_INSTALL_DIR:-\$HOME/.local/bin}"
      printf '  Override the destination with KEMPT_INSTALL_DIR.\n'
      exit 0 ;;
    *) fail "unknown option '$arg' (see --help)" ;;
  esac
done

command -v curl >/dev/null 2>&1 || fail "curl is required"
command -v tar  >/dev/null 2>&1 || fail "tar is required"

os="$(uname -s | tr 'A-Z' 'a-z')"
case "$os" in
  darwin|linux) ;;
  *) fail "unsupported OS '$os' — kempt runs on macOS and Linux (on Windows, install inside WSL2)" ;;
esac

arch="$(uname -m)"
case "$arch" in
  x86_64|amd64)  arch="amd64" ;;
  arm64|aarch64) arch="arm64" ;;
  *) fail "unsupported architecture '$arch' (need amd64 or arm64)" ;;
esac

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

asset="kempt_${os}_${arch}.tar.gz"

curl -fsSL "$base/checksums.txt" -o "$tmp/checksums.txt" || fail "download failed: checksums.txt"

printf 'downloading %s …\n' "$asset"
curl -fsSL "$base/$asset" -o "$tmp/$asset" || fail "download failed: $base/$asset"

# sha256 the given file, using whichever tool this OS ships (shasum on macOS,
# sha256sum on Linux).
sha256_of() {
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | cut -d' ' -f1
  else
    sha256sum "$1" | cut -d' ' -f1
  fi
}

expected="$(grep " $asset\$" "$tmp/checksums.txt" | cut -d' ' -f1)"
[ -n "$expected" ] || fail "no checksum found for $asset"
actual="$(sha256_of "$tmp/$asset")"
[ "$actual" = "$expected" ] || fail "checksum mismatch for $asset (expected $expected, got $actual)"

mkdir -p "$install_dir"
tar -xzf "$tmp/$asset" -C "$tmp"
mv "$tmp/kempt" "$install_dir/kempt"
chmod +x "$install_dir/kempt"
printf 'installed kempt to %s\n' "$install_dir/kempt"

# Smoke test: `kempt version` exits 0, confirming the binary loads and runs on
# this machine without touching anything.
"$install_dir/kempt" version >/dev/null 2>&1 || fail "installed binary failed to run"

case ":$PATH:" in
  *":$install_dir:"*) ;;
  *) printf 'note: %s is not on your PATH — add:  export PATH="%s:$PATH"\n' "$install_dir" "$install_dir" ;;
esac
printf 'next: kempt init <your-config-repo-url>\n'
