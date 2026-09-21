# kempt doctor — machine-vs-manifest health & drift detection

Status: **design, pending review** · 2026-09-21

## Problem

kempt converges a machine to a manifest, but it has a structural **blind spot**:
it cannot tell you when a machine has drifted in ways the manifest's own
tolerant semantics deliberately ignore.

Concrete failure that motivated this (real, observed): a pi `settings.json`
`packages` array accumulated to **29 entries with 9 duplicate identities**
(`pi-quiet` *and* `pi-quiet@0.2.0`, …). Two writers touched one array — kempt's
`json-merge` and `pi install` (which stamps resolved versions) — and the
`json-merge` used `arrays = "append"`. Nothing surfaced it:

- **`kempt plan` stayed green.** `jsonMergeHandler.Inspect` in append mode calls
  `isSubset(desired, current, replace=false)`, which returns `OpNoop` as long as
  every *desired* element is present. Extra elements in the live file are
  invisible to it. (Confirmed in `internal/engine/handlers/jsonmerge.go`.)
- **`kempt verify` doesn't look.** It only runs explicitly-declared `verify`
  steps (`command-exists`, `symlink-target`, …).
- **`install` never removes.** `piInspect`/`npmInspect` install what's *missing*;
  software present on the machine but absent from the manifest is never reported.

Switching that one array to `arrays = "replace"` fixed *that instance* (plan now
diffs it), but the general classes — append-superset drift, duplicate
identities, orphaned installs, foreign symlinks, and the human-only
"install list must match the settings-merge string-for-string" rule — remain
undetectable by design. `doctor` closes the gap: one read-only command that
answers **"is this machine actually what the manifest says, and is everything
using the correct kempt install?"**

## Goals

- A standalone **`kempt doctor`** command: read-only, exit-coded, `--json`, that
  reports machine-vs-manifest divergence the existing commands cannot.
- Catch, at minimum: extra-array drift (even in append merges), duplicate
  identities within a managed array, orphaned installed software, and
  broken/foreign managed symlinks — plus a `plan`/`verify` rollup so `doctor` is
  the single "health" entrypoint.
- Move the one **manifest-internal** invariant (install list ↔ settings-merge
  `packages`, string-for-string) into **`kempt lint`**, so a bad manifest fails
  offline before it ever reaches a machine.
- Be safe by construction: **report-only in v1**, every finding carries a
  concrete remediation string. No mutation.

## Non-goals (v1)

- `--fix` / auto-remediation. Deferred to a fast-follow; the remediation for the
  drift class is already `kempt apply` (replace heals) or `kempt adopt <pkg>`.
- Replacing `plan`/`verify`. `doctor` *aggregates* them, it does not supersede.
- Network version checks — that is `outdated`'s job; `doctor` is offline except
  for the inventories it shells out for (`pi list`, `npm ls -g`), same as
  `install` inspection today.

## Command surface

```
kempt doctor [flags]
  -manifest string   path to manifest        (same resolution as plan/verify)
  -profile  string   profile to select
  -packages string   comma-separated package names
  -json              emit machine-readable findings
  -strict            treat warn-level findings as failures for the exit code
```

Selection resolves identically to `plan`/`verify` (saved selection when flags
omitted), via the existing `loadSelectedContext` / `resolveManifest` /
`resolveSelection` helpers.

### Exit codes

- `0` — healthy: no `error`- or (under `-strict`) `warn`-level findings.
- `1` — findings present at the failing threshold.
- `2` — usage/parse error (`UsageError`, consistent with other commands).

Default failing threshold is **error**; `-strict` lowers it to **warn** so CI
can gate hard. `info` never fails the exit code.

## Checks (the B / machine-health set)

Each check yields zero or more **Findings**. A Finding has: `check` id,
`severity` (`error|warn|info`), `package` (when attributable), `detail`, and a
`remediation` string. All checks are read-only.

### D1 — extra-array drift  (severity: warn; error under `-strict` when the step is `arrays=replace`)
For every `json-merge` step in the selected packages: load the live file, and
for each array the manifest writes, report **live elements the manifest does not
declare**. This is the inverse of `isSubset` and is reported *regardless of
append/replace* — append is legal but drift-prone, so it is a `warn`; a
`replace` step with extras is a genuine inconsistency (`error`).
- **Reuses:** `jsonmerge`'s `expandHome`/`toAny` and a new `arrayExtras(desired,
  current)` walk parallel to `isSubset`.
- **Remediation:** "N undeclared entries in `<file>` `<jsonpath>`; set
  `arrays = \"replace\"` on this json-merge and run `kempt apply`, or add them to
  the manifest."

### D2 — duplicate identity in a managed array  (severity: warn)
Within each manifest-managed live array, detect entries that normalize to the
same **identity**: strip a trailing `@version` and the `npm:`/`git:` scheme
(`npm:pi-quiet@0.2.0` ≡ `npm:pi-quiet`); git identity is the repo URL without
`@ref`; local is the resolved path. Report each identity present more than once.
- **Remediation:** "`<file>` lists `<id>` N times (`<variants>`); run
  `kempt apply` (an `arrays=replace` merge collapses these) or dedupe the file."

### D3 — orphaned installed software  (severity: warn for pi; info for npm)
Compare live inventories against the manifest's declared installs:
- **pi:** `piInventory` (`pi list`) vs the union of `install.pi` specs → report
  registered pi packages not declared by any selected package.
- **npm:** `npmInventory` (`npm ls -g`) vs `install.npm` → report globals not
  declared. **Off by default** (global npm is noisy with hand-installed tools);
  opt in via `[doctor] checkNpmOrphans = true`.
- **Reuses:** `piInventory`/`npmInventory` in `internal/engine/handlers/install.go`
  (promote to a shared, memoized inventory package or export).
- **Ignore list:** manifest `[doctor] ignore = ["npm:some-tool", "glob:*"]`
  suppresses known-intentional out-of-band installs.
- **Remediation:** "`<pkg>` is installed but not in the manifest; `kempt adopt`
  it into a package to keep it, or remove it (`pi remove <pkg>` / manually)."

### D4 — broken or foreign managed symlinks  (severity: error broken; warn foreign)
For every `symlink` step: `lstat` the `to` path. Report **broken** (dangling
target), **foreign** (a real file/dir where the repo symlink is expected — the
`backup`-would-fire case), or **mispointed** (symlink to a different target).
Complements `verify`'s `symlink-target` by covering links that have *no* verify
step. Reuses the `symlink` handler's inspection logic.
- **Remediation:** "`<to>` is <state>; run `kempt apply` (it will `backup` and
  relink)."

### D5 — plan / verify rollup  (severity: info, or mirrors underlying)
Run `engine.BuildPlan` and the `verify` steps for the selection and summarize:
pending `OpChange` count, `OpBlocked` count, verify pass/fail. Surfaces ordinary
"machine is behind the manifest" state in the same report. Blocked steps and
verify failures are promoted to `error`.
- **Reuses:** `engine.BuildPlan`, the `verify` handler loop from `verifycmd.go`.

## lint addition (offline, manifest-internal)

### L1 — install list ↔ settings-merge packages string-for-string  (lint finding)
Today `kempt.toml` enforces "the settings `packages` merge MUST match the
install `pi = [...]` string-for-string" with a **comment and human discipline
only**. Move it into `manifest.Validate` (surfaced by `kempt lint` and the
verify preflight): when one package declares both an `install.pi` list and a
`json-merge` whose `file` is the pi settings and whose `merge.packages` is an
array, assert the two lists are equal (order-insensitive set equality, exact
strings). Emit a `Finding` with the offending package + first divergence.
- **Why lint, not doctor:** it compares two halves of the manifest and needs no
  machine, so it should fail in CI/offline before reaching a box.
- Generalize modestly: the rule keys on "an `install.<backend>` list and a
  `json-merge` writing that backend's registry array in the same package,"
  so it also covers a future npm equivalent.

## Output

**Human (default):** grouped by severity, each line `  <sev> <check> <detail>`
then an indented `→ <remediation>`, ending with a one-line summary
(`N errors, M warnings, K info`) or `healthy: no findings`.

**`--json`:** `{ "findings": [ {check, severity, package, detail, remediation} ],
"summary": {error, warn, info} }` — stable shape for automation.

## Architecture / implementation sketch

- `internal/cli/doctor.go` — command registration (mirrors `verifycmd.go`),
  flag parsing, selection, orchestration, rendering, exit mapping.
- `internal/doctor/` (new package) — the checks as pure functions over
  `(*machine.Context, []*manifest.Package)` returning `[]Finding`, so they are
  unit-testable without the CLI. One file per check (`d1_extra_array.go`, …)
  plus `finding.go` (types + severity) and `report.go` (human/json render).
- **Reuse, don't duplicate:**
  - array/subset logic: factor `arrayExtras` + identity-normalize helpers
    alongside `jsonmerge.go` (or a small shared `internal/jsonutil`).
  - inventories: export/promote `piInventory`/`npmInventory` from the install
    handler into a shared spot both `install` and `doctor` import.
  - `engine.BuildPlan`, `verify` handler, `symlink` inspection for D4/D5.
- `internal/manifest/validate.go` — add L1.
- New manifest block `[doctor]` (`checkNpmOrphans bool`, `ignore []string`) in
  `internal/manifest/types.go` + schema (`internal/schema`).
- Register `doctor` in `internal/cli/cli.go` command table; add to
  `commands`/help and the `--json` command index.

## Testing

- **Unit (table-driven), per check** in `internal/doctor`:
  - D1: append merge with extra live entries → warn; replace with extras →
    error; clean → none.
  - D2: `["npm:x","npm:x@1.0.0"]` → one dup finding; distinct → none; git
    ref/no-ref identity; local path identity.
  - D3: fake `piInventory`/`npmInventory` (existing `FakeRunner`) with declared
    + orphan sets; ignore-list suppression; npm-orphans gated off by default.
  - D4: broken / foreign-file / mispointed / correct symlink fixtures.
  - D5: plan with an `OpChange`/`OpBlocked`; verify pass/fail.
- **L1** in `manifest` tests: matching lists → no finding; reordered → none
  (set-equal); missing/extra/version-mismatch → finding naming the package.
- **CLI** in `internal/cli`: exit `0` clean, `1` with findings, `1` under
  `-strict` with only warns, `2` on bad flags; `--json` shape golden.

## Rollout

1. Implement `doctor` + L1 + `[doctor]` schema, behind no flag (new command is
   additive).
2. Docs: new `## doctor` section in `docs/spec.md` command list + README
   "day-to-day" table row; `kempt help doctor`; man page.
3. Wire into the repo's own CI if kempt runs kempt on itself.
4. **This repo's dogfooding:** after release, the drift that started this
   (dotfiles pi packages) is caught by `doctor` on every machine; the
   `arrays=replace` change already prevents it, and L1 guards the
   string-for-string rule at lint time.

## Open questions / future

- **`--fix`** (fast-follow): map each finding class to a safe action — drift/dup
  → `kempt apply`; orphan → prompt `adopt` vs prune; foreign symlink → apply
  with backup. Gated, prompted, never silent.
- Should `refresh`/`status` embed a doctor summary so drift shows in the prompt
  line without a separate invocation?
- Orphan provenance: kempt cannot always know it installed a package; the
  ignore-list + off-by-default npm is the v1 answer. A future install-ledger
  could make it exact.
