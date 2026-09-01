# The kempt standard (`kempt.toml`)

A kempt config repo is one `kempt.toml` plus the config files it references.
The manifest is **fully declarative**: kempt executes only built-in, versioned
primitives. There is no exec/script escape hatch. Every action is reviewable
before it runs.

This document is the standard, versioned by the `spec` field.

## Shape

```toml
[kempt]
spec = 1

[packages.core]
description = "shell, prompt, history, modern CLI tools"

  [[packages.core.install]]
  brew   = { formulas = ["starship", "atuin", "ripgrep", "jq"] }
  winget = ["BurntSushi.ripgrep.MSVC", "jqlang.jq"]
  apt    = ["ripgrep", "jq"]

  [[packages.core.symlink]]
  from = "config/zsh"
  to = "~/.config/zsh"
  backup = true

[packages.terminal]
description = "Ghostty + tmux + yazi workspace"
needs = ["core"]

  [[packages.terminal.git-clone]]
  repo = "https://github.com/tmux-plugins/tmux-resurrect"
  to = "~/.tmux/plugins/tmux-resurrect"

[packages.muster]
description = "local multi-agent coordination bus"
only = { os = "darwin" }

  [[packages.muster.github-release]]
  repo = "schuettc/muster"
  asset = "muster_{os}_{arch}.tar.gz"
  bin = "muster"

  [[packages.muster.json-merge]]
  file = "~/.claude/settings.json"
  merge = { mcpServers.muster = { command = "muster", args = ["mcp"] } }

  [[packages.muster.service]]
  label = "tools.muster.serve"
  program = ["muster", "serve"]

  [[packages.muster.verify]]
  command-exists = "muster"
  version-current = { repo = "schuettc/muster", command = "muster version" }

[profiles.developer]
description = "full toolchain: terminal, editors, agents"
packages = ["core", "terminal", "muster"]

[profiles.minimal]
description = "just a pleasant shell"
packages = ["core"]
```

## Primitives (v1, closed set)

The primitive set is closed. When a real capability gap appears, the answer is
a new versioned primitive in kempt, not a script hook.

| Primitive | Safety class | Notes |
|---|---|---|
| `install` | software | Per-OS/cross-platform backends: `brew` (formulas/casks/taps), `winget`, `apt`, `npm` (global packages), `pi` (more later). Backends are additive — `npm`/`pi` install on any host that has the runtime, alongside the OS backend. Kempt selects the applicable backends at plan time; a platform with no matching backend skips or plan-errors per `only`/`needs`. `install.npm` and `install.pi` entries may be **pinned** as `name@version` (version embedded in the string — no schema change): kempt then converges to that exact version, reinstalling on `kempt update` when the installed version differs. Unversioned entries stay presence-only (satisfied by any installed version). |
| `github-release` | software | Asset templating `{os}`/`{arch}`; downloads asset + `checksums.txt`, sha256-verifies, stages to `.$bin.new.$$`, atomic `mv -f` into `~/.local/bin` (running daemons keep their inode). |
| `download` | software | Fetch a binary from a domain distribution (not GitHub). Resolves `version` (`latest` by default, or a pinned semver), downloads the family URL, verifies its `.sha256` sidecar, and installs `bin` into `~/.local/bin` via the same stage-and-atomic-`mv` path as `github-release`. See the family-URL contract below. |
| `git-clone` | software | Pinned ref or branch. |
| `service` | software | Backend: launchd (macOS), systemd `--user` (Linux); Windows later. Renders unit/plist, `cmp`-before-reload so unchanged services never restart. Fields: `label`, `program` (argv), plus optional `env`, `stdout`, `stderr`, `keep-alive`, `run-at-load`, `process-type`, `throttle-interval`, `session-type` (mapped to the backend's native keys). |
| `symlink` | files | Repo-relative `from`; `backup = true` moves a real file to `.bak` first. Windows requires Developer Mode — detected and reported in plan, not failed mid-apply. |
| `json-merge` | files | Additive deep merge (jq semantics); idempotent; multiple packages may merge into the same file. `arrays` selects list behavior: `append` (default, union) or `replace` (desired list wins). Covers MCP registration and harness hooks. The literal token `${HOME}` in any merged string value is expanded to the machine's home at write time — for files that do not themselves expand `~` (e.g. codex `hooks.json`, whose command strings need absolute paths). Bare `~` is left untouched. |
| `toml-merge` | files | Additive deep merge for TOML config files (e.g. an agent's `config.toml`); idempotent; maps recurse, arrays append-missing, scalars overwrite. Expands the `${HOME}` token in string values like `json-merge`. |
| `line-in-file` | files | Ensure-line/ensure-block for shell includes and PATH nudges. |
| `verify` | read-only | Declared checks: `command-exists`, `command-exists-any` (any of a list on PATH), `http-ok` (URL returns 2xx), symlink target, `version-current` (GitHub latest-release drift). |

### `download` family-URL contract

The `download` primitive resolves assets from a predictable URL family so a
vendor need only publish files at these paths:

- **Artifact:** `https://<site>/dl/<tool>/<ver>/<tool>_<os>_<arch>.tar.gz`
- **Latest pointer:** a plain-text document holding a bare semver (e.g. `1.4.2`),
  used when `version` is `latest` or omitted, to resolve `<ver>`.
- **Checksum:** a shasum-format `.sha256` sidecar alongside the artifact
  (`<artifact>.sha256`), verified before install.

`<os>`/`<arch>` use kempt's normalized platform tokens (same as `github-release`
asset templating).

Safety class comes from kempt's primitive table — a manifest cannot declare its
own software installs "safe."

Every package may carry a `notes` field — an array of free-form strings surfaced
in plan output (post-install hints, manual follow-ups). Notes are documentation
only; they run nothing.

## Rules

- Steps run in written order within a package; packages run in dependency
  (`needs`) order. Planned for a later spec revision, not part of spec = 1: step-level
  `requires` preflights (e.g. "git exists") that fail the plan, not the apply. A spec = 1
  parser rejects `requires` as an unknown key.
- Every step is idempotent by contract: kempt computes current vs desired and
  no-ops on match. This is what makes plan honest and converge cheap.
- `only = { os, arch }` on packages or steps restricts where they apply; always
  visible in plan output.

## Profiles

Profiles are named package sets (personas: developer, minimal, …), **not**
conditionals. The first-run picker preseeds from the chosen profile. There is no
hostname/env conditional language in v1 — `only = { os, arch }` is the only
conditional.

## Versioning

The manifest declares `spec = 1`. Parsers reject spec values they don't know:
the current engine's `Validate` enforces `spec = 1` and flags anything else.
Future spec revisions bump this field, and older engines refuse manifests from
the future rather than mis-parse them. A published JSON Schema gives TOML LSPs
(taplo) completion and validation while authoring.

## Implementation status

- `kempt lint` — shipped (unknown keys, spec validation, structural checks).
- `kempt plan` / `apply` / `init` / `update` — in progress (phases 1b–1c).
