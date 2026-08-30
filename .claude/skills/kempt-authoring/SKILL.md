---
name: kempt-authoring
description: Use when helping someone author or evolve a kempt.toml manifest — turning a machine's current setup into a reviewable declarative config, adding software or dotfiles to an existing manifest, or debugging why `kempt plan` shows changes that should be no-ops. Fires when the `kempt` CLI is available and the task is writing/editing kempt.toml, not operating an already-converged machine.
---

# Authoring a kempt.toml

kempt is declarative machine setup you can read before you run. A config repo is
one `kempt.toml` plus the files it references; kempt parses it, computes the diff
against the real machine, and applies only what the user approves. The primitive
set is **closed** and there is **no exec/script escape hatch** — so your job when
authoring is to express intent with built-in primitives, never to reach for a
shell hook that doesn't exist.

## The loop: dump → curate → lint → plan → apply

1. **Start from reality, not a blank page.** `kempt dump` inspects the current
   machine and prints a suggested manifest (read-only — it changes nothing, so
   run it freely). `kempt new` scaffolds an empty manifest when there's nothing
   to harvest. Treat `dump` output as a draft to curate, not a finished file.
2. **Curate.** Group steps into `[packages.<name>]`; keep only what the user
   actually wants reproduced. Steps within a package run in **written order** —
   order them so dependencies come first (install a binary before the service
   that runs it).
3. **`kempt lint`** validates structure and reports unknown keys / rule
   violations deterministically. `kempt schema` prints the authoritative JSON
   Schema — consult it rather than guessing field names.
4. **`kempt plan`** shows exactly what `apply` would change, read-only. A clean
   manifest on an already-set-up machine should show **all no-ops**. Any
   surprise change is a signal the manifest doesn't match reality yet.
5. **`kempt apply`** converges the machine — only after the user has read the
   plan. Never apply without a plan the user has seen.

## The primitives (closed set — pick from these, never invent)

Software class: `install` (backends `brew` / `winget` / `apt` / `npm` / `pi` —
additive; `npm`/`pi` entries may be pinned `name@version` to converge to an exact
version), `github-release` (asset templating + `checksums.txt` verify + atomic
install), `download` (fetch from a domain distribution + `.sha256` sidecar),
`git-clone` (pinned ref), `service` (launchd / systemd `--user`).

Files class: `symlink` (repo-relative `from`, optional `backup`), `json-merge`
(deep merge, `arrays = append|replace`), `toml-merge` (deep merge for TOML),
`line-in-file` (ensure-line / ensure-block).

Read-only: `verify` (`command-exists`, `command-exists-any`, `http-ok`, symlink
target, `version-current`). And `notes` — free-text follow-up hints surfaced in
plan output (use for manual steps kempt can't do, e.g. a GUI permission toggle).

**When a real capability is missing, the answer is a new versioned primitive in
kempt — not a workaround.** Do not model an exec/script step; there is none by
design, and that absence is the whole trust story. Say so plainly to the user and
file it as a kempt feature request.

## Selection vs. manifest

`kempt adopt <pkg>` / `kempt drop <pkg>` edit which packages this machine has
**selected** (saved machine state), never the manifest file. Use them to turn a
package on/off for a machine; edit `kempt.toml` to change what a package *is*.

## Conditionals and profiles

Gate a package or step with `only = { os = "...", arch = "..." }` for
platform-specific pieces, and `needs = ["otherpkg"]` for ordering across
packages. `[profiles.<name>]` presets a selection (e.g. `developer`, `minimal`)
that seeds the picker — a persona, not a separate config.

## Ongoing operation (not authoring, but worth knowing)

`kempt status` summarizes drift; `kempt refresh` re-checks and applies the
files-class updates that are opt-in; `kempt update` pulls the config repo and
converges everything (split update policy — software changes only ever apply
through `update`/`apply`, never silently). `kempt init <repo-url>` clones a
manifest repo onto a fresh machine.

## Debugging "plan shows a change that should be a no-op"

- A `symlink` "retarget" on every run usually means a **relative** manifest path
  resolved against the wrong base — prefer an absolute `-manifest` path.
- An `install` reinstall on a pinned `npm`/`pi` entry means the installed version
  differs from the pin — that's correct behavior; bump the pin or the machine.
- A `json-merge`/`toml-merge` that never settles means a value kempt writes
  differs from what's on disk — inspect the target file against the merge subtree.
