# kempt doctor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a read-only `kempt doctor` command (plus one offline `kempt lint` invariant) that reports machine-vs-manifest drift the existing `plan`/`verify` structurally miss.

**Architecture:** Pure check functions in a new `internal/doctor` package over `(*machine.Context, []*manifest.Package, manifest.DoctorConfig) → []Finding`, composed by `doctor.Run`. A thin `internal/cli/doctor.go` registers the command, resolves selection like `verify`, renders human/JSON, and maps findings to an exit code. Shared helpers (spec-identity, array-extras, inventories) are factored out so `doctor` reuses — not forks — existing logic.

**Tech Stack:** Go (stdlib only), `github.com/schuettc/tools-common` for `Command`, existing `internal/engine`, `internal/manifest`, `internal/machine`, `internal/run` (FakeRunner) for tests.

**Spec:** `docs/specs/2026-09-21-kempt-doctor.md` — read it alongside this plan.

## Global Constraints

- Go stdlib only; no new third-party dependencies.
- All `doctor` checks are **strictly read-only** (like `Handler.Inspect`). No file writes, no `Apply`.
- Commands register via `init()` calling `Register(Command{...})` (alias of `tools.Command`); the file living in package `cli` is enough — no `cli.go` edit needed.
- Exit codes: `0` healthy, `1` findings at threshold, `2` usage error (`UsageError`). Default threshold = `error`; `-strict` lowers to `warn`; `info` never fails.
- Selection resolves exactly like `verify`: `loadState` → `resolveManifest` → `resolveSelection` → `loadManifestSource` → `manifest.Parse` + `manifest.Validate` → `engine.Select`.
- Every finding carries a concrete `Remediation` string. v1 is report-only; **no `--fix`**.
- Follow existing test idiom: table-driven, `run.FakeRunner{Responses: map[string]run.Response{...}}`, keys like `"lookpath pi"`, `"pi list"`.

---

### Task 1: Spec-identity normalization (shared helper)

**Files:**
- Create: `internal/jsonutil/identity.go`
- Test: `internal/jsonutil/identity_test.go`

**Interfaces:**
- Produces: `func SpecIdentity(spec string) string` — collapses a package spec to its identity: strips a leading `npm:`/`git:` scheme, and for npm strips a trailing `@version` (but not the `@` of an `@scope/name`); for git strips a trailing `@ref`; local paths return unchanged.

- [ ] **Step 1: Write the failing test**

```go
package jsonutil

import "testing"

func TestSpecIdentity(t *testing.T) {
	cases := map[string]string{
		"npm:pi-quiet":                      "pi-quiet",
		"npm:pi-quiet@0.2.0":                "pi-quiet",
		"npm:@scope/name":                   "@scope/name",
		"npm:@scope/name@1.2.3":             "@scope/name",
		"git:github.com/obra/superpowers":   "github.com/obra/superpowers",
		"git:github.com/obra/superpowers@v1": "github.com/obra/superpowers",
		"/abs/local/path":                   "/abs/local/path",
	}
	for in, want := range cases {
		if got := SpecIdentity(in); got != want {
			t.Errorf("SpecIdentity(%q) = %q; want %q", in, got, want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/jsonutil/ -run TestSpecIdentity -v`
Expected: FAIL — `undefined: SpecIdentity`.

- [ ] **Step 3: Write minimal implementation**

```go
// Package jsonutil holds small pure helpers for comparing manifest-desired
// JSON against live files, shared by json-merge inspection and doctor checks.
package jsonutil

import "strings"

// SpecIdentity collapses a kempt package spec to its identity: scheme and
// version/ref stripped, so "npm:pi-quiet@0.2.0" and "npm:pi-quiet" match. An
// @scope/name keeps its leading @; only a trailing @segment is a version/ref.
func SpecIdentity(spec string) string {
	s := spec
	switch {
	case strings.HasPrefix(s, "npm:"):
		s = strings.TrimPrefix(s, "npm:")
		if at := strings.LastIndex(s, "@"); at > 0 { // >0 keeps @scope
			s = s[:at]
		}
	case strings.HasPrefix(s, "git:"):
		s = strings.TrimPrefix(s, "git:")
		if at := strings.LastIndex(s, "@"); at > 0 {
			s = s[:at]
		}
	}
	return s
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/jsonutil/ -run TestSpecIdentity -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/jsonutil/identity.go internal/jsonutil/identity_test.go
git commit -m "feat(jsonutil): SpecIdentity helper for package-spec identity"
```

---

### Task 2: Array-extras helper (live minus desired)

**Files:**
- Create: `internal/jsonutil/extras.go`
- Test: `internal/jsonutil/extras_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces: `func ArrayExtras(desired, current []any) []any` — elements of `current` not deeply equal to any element of `desired`, order preserved. This is the inverse of json-merge's append-mode subset test and is what makes bloat visible.

- [ ] **Step 1: Write the failing test**

```go
package jsonutil

import (
	"reflect"
	"testing"
)

func TestArrayExtras(t *testing.T) {
	desired := []any{"npm:a", "npm:b"}
	current := []any{"npm:a", "npm:a@1.0.0", "npm:b", "npm:c"}
	got := ArrayExtras(desired, current)
	want := []any{"npm:a@1.0.0", "npm:c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ArrayExtras = %v; want %v", got, want)
	}
	if e := ArrayExtras([]any{"x"}, []any{"x"}); len(e) != 0 {
		t.Fatalf("no extras expected, got %v", e)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/jsonutil/ -run TestArrayExtras -v`
Expected: FAIL — `undefined: ArrayExtras`.

- [ ] **Step 3: Write minimal implementation**

```go
package jsonutil

import "reflect"

// ArrayExtras returns the elements of current that do not deep-equal any
// element of desired, preserving current's order. Used by doctor to surface
// live entries the manifest never declared (the append-accumulation signature).
func ArrayExtras(desired, current []any) []any {
	var extras []any
	for _, c := range current {
		found := false
		for _, d := range desired {
			if reflect.DeepEqual(c, d) {
				found = true
				break
			}
		}
		if !found {
			extras = append(extras, c)
		}
	}
	return extras
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/jsonutil/ -run TestArrayExtras -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/jsonutil/extras.go internal/jsonutil/extras_test.go
git commit -m "feat(jsonutil): ArrayExtras helper (live entries absent from manifest)"
```

---

### Task 3: Extract inventories into a shared package

**Files:**
- Create: `internal/inventory/inventory.go`
- Test: `internal/inventory/inventory_test.go`
- Modify: `internal/engine/handlers/install.go` (delegate `piInventory`/`npmInventory` to the new package; keep behavior identical)

**Interfaces:**
- Produces: `func Pi(ctx *machine.Context) (map[string]string, error)` and `func Npm(ctx *machine.Context) (map[string]string, error)` — name→version maps, memoized via `ctx.Cache`, identical parsing to today's `piInventory`/`npmInventory` (tolerates `pi list` section headers; preserves `@scope/name` for npm).
- Consumes (in install.go): the new `inventory.Pi`/`inventory.Npm`.

- [ ] **Step 1: Write the failing test** (move the two behaviors under test in the new package)

```go
package inventory

import (
	"testing"

	"github.com/schuettc/kempt/internal/machine"
	"github.com/schuettc/kempt/internal/run"
)

func ctxWith(fake *run.FakeRunner) *machine.Context {
	c, _ := machine.New("/r", fake)
	c.Home = "/h"
	return c
}

func TestPiInventorySkipsHeaders(t *testing.T) {
	c := ctxWith(&run.FakeRunner{Responses: map[string]run.Response{
		"pi list": {Stdout: "User packages:\n  npm:typescript@5.0.0\n  /Users/me/local-tool\n"},
	}})
	inv, err := Pi(c)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := inv["User packages:"]; ok {
		t.Fatal("header must not be an entry")
	}
	if inv["npm:typescript"] != "5.0.0" {
		t.Fatalf("typescript version = %q", inv["npm:typescript"])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/inventory/ -v`
Expected: FAIL — package/functions undefined.

- [ ] **Step 3: Move the implementations**

Cut `piInventory`, `npmInventory`, their command-string consts (`piInventoryCmd`, `npmInventoryCmd`/`npmInvCmd`), and their parsing helpers out of `internal/engine/handlers/install.go` into `internal/inventory/inventory.go`, renaming the exported entry points to `Pi` and `Npm` (keep parsing byte-for-byte). In `install.go`, replace call sites with `inventory.Pi(ctx)` / `inventory.Npm(ctx)` and import the new package. Preserve the `ctx.Cache` memoization keys exactly (so existing install tests still hit the cache).

- [ ] **Step 4: Run the full suite to verify nothing regressed**

Run: `go test ./...`
Expected: PASS (both new `inventory` tests and all existing `handlers` install tests).

- [ ] **Step 5: Commit**

```bash
git add internal/inventory/ internal/engine/handlers/install.go
git commit -m "refactor(inventory): extract Pi/Npm inventories into shared package"
```

---

### Task 4: Finding type, severity, and report rendering

**Files:**
- Create: `internal/doctor/finding.go`
- Create: `internal/doctor/report.go`
- Test: `internal/doctor/report_test.go`

**Interfaces:**
- Produces:
  - `type Severity int` with `Info Severity = iota; Warn; Error`; `func (Severity) String() string` → `"info"|"warn"|"error"`.
  - `type Finding struct { Check, Package, Detail, Remediation string; Severity Severity }`
  - `type Report struct { Findings []Finding }`
  - `func (r Report) Counts() (errs, warns, infos int)`
  - `func (r Report) Failed(strict bool) bool` — true if any `Error`, or any `Warn` when `strict`.
  - `func (r Report) WriteHuman(w io.Writer)` and `func (r Report) WriteJSON(w io.Writer) error`.

- [ ] **Step 1: Write the failing test**

```go
package doctor

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestReportCountsAndFailed(t *testing.T) {
	r := Report{Findings: []Finding{
		{Check: "d1", Severity: Warn, Detail: "x", Remediation: "y"},
		{Check: "d4", Severity: Error, Detail: "z", Remediation: "w"},
	}}
	e, wn, i := r.Counts()
	if e != 1 || wn != 1 || i != 0 {
		t.Fatalf("counts = %d,%d,%d", e, wn, i)
	}
	if !r.Failed(false) {
		t.Fatal("error should fail default threshold")
	}
	warnOnly := Report{Findings: []Finding{{Severity: Warn}}}
	if warnOnly.Failed(false) {
		t.Fatal("warn must not fail default threshold")
	}
	if !warnOnly.Failed(true) {
		t.Fatal("warn must fail under strict")
	}
}

func TestReportJSON(t *testing.T) {
	var b bytes.Buffer
	r := Report{Findings: []Finding{{Check: "d2", Severity: Warn, Package: "pi", Detail: "dup", Remediation: "apply"}}}
	if err := r.WriteJSON(&b); err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Findings []map[string]any `json:"findings"`
		Summary  map[string]int   `json:"summary"`
	}
	if err := json.Unmarshal(b.Bytes(), &doc); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	if doc.Findings[0]["severity"] != "warn" || doc.Summary["warn"] != 1 {
		t.Fatalf("bad json: %s", b.String())
	}
}

func TestReportHumanHealthy(t *testing.T) {
	var b bytes.Buffer
	Report{}.WriteHuman(&b)
	if !strings.Contains(b.String(), "healthy: no findings") {
		t.Fatalf("want healthy line, got %q", b.String())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/doctor/ -v`
Expected: FAIL — undefined types.

- [ ] **Step 3: Write minimal implementation**

`finding.go`:

```go
// Package doctor holds read-only machine-vs-manifest health checks composed by
// Run and surfaced by the `kempt doctor` command.
package doctor

type Severity int

const (
	Info Severity = iota
	Warn
	Error
)

func (s Severity) String() string {
	switch s {
	case Error:
		return "error"
	case Warn:
		return "warn"
	default:
		return "info"
	}
}

// Finding is one read-only diagnosis. Remediation is a concrete next step.
type Finding struct {
	Check       string
	Package     string
	Detail      string
	Remediation string
	Severity    Severity
}
```

`report.go`:

```go
package doctor

import (
	"encoding/json"
	"fmt"
	"io"
)

type Report struct{ Findings []Finding }

func (r Report) Counts() (errs, warns, infos int) {
	for _, f := range r.Findings {
		switch f.Severity {
		case Error:
			errs++
		case Warn:
			warns++
		default:
			infos++
		}
	}
	return
}

// Failed reports whether the run should exit non-zero: any error, or any warn
// when strict.
func (r Report) Failed(strict bool) bool {
	e, w, _ := r.Counts()
	if e > 0 {
		return true
	}
	return strict && w > 0
}

func (r Report) WriteHuman(w io.Writer) {
	if len(r.Findings) == 0 {
		fmt.Fprintln(w, "healthy: no findings")
		return
	}
	for _, sev := range []Severity{Error, Warn, Info} {
		for _, f := range r.Findings {
			if f.Severity != sev {
				continue
			}
			pkg := ""
			if f.Package != "" {
				pkg = " [" + f.Package + "]"
			}
			fmt.Fprintf(w, "  %-5s %s%s %s\n", f.Severity, f.Check, pkg, f.Detail)
			fmt.Fprintf(w, "        → %s\n", f.Remediation)
		}
	}
	e, wn, i := r.Counts()
	fmt.Fprintf(w, "%d errors, %d warnings, %d info\n", e, wn, i)
}

func (r Report) WriteJSON(w io.Writer) error {
	e, wn, i := r.Counts()
	type jf struct {
		Check       string `json:"check"`
		Severity    string `json:"severity"`
		Package     string `json:"package,omitempty"`
		Detail      string `json:"detail"`
		Remediation string `json:"remediation"`
	}
	out := struct {
		Findings []jf           `json:"findings"`
		Summary  map[string]int `json:"summary"`
	}{Summary: map[string]int{"error": e, "warn": wn, "info": i}}
	for _, f := range r.Findings {
		out.Findings = append(out.Findings, jf{f.Check, f.Severity.String(), f.Package, f.Detail, f.Remediation})
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/doctor/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/doctor/finding.go internal/doctor/report.go internal/doctor/report_test.go
git commit -m "feat(doctor): Finding/Severity types and human+json report"
```

---

### Task 5: D1 — extra-array drift check

**Files:**
- Create: `internal/doctor/d1_extra_array.go`
- Test: `internal/doctor/d1_extra_array_test.go`

**Interfaces:**
- Consumes: `jsonutil.ArrayExtras`; manifest `JSONMergeStep` (`File string`, `Merge map[string]any`, `Arrays string`); `machine.Context.Expand`, `.Home`.
- Produces: `func CheckExtraArray(ctx *machine.Context, pkgs []*manifest.Package) []Finding` — for each `json-merge` step, for each **top-level** key in `Merge` whose value is an array, compare against the live file's same key; report extras. `warn` normally, `error` when the step is `arrays = "replace"` (extras there are a hard inconsistency). Missing/invalid live file → no finding (that's `plan`'s job).

- [ ] **Step 1: Write the failing test**

```go
package doctor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/schuettc/kempt/internal/machine"
	"github.com/schuettc/kempt/internal/manifest"
	"github.com/schuettc/kempt/internal/run"
)

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func ctxT(t *testing.T) *machine.Context {
	c, _ := machine.New(t.TempDir(), &run.FakeRunner{})
	c.Home = t.TempDir()
	return c
}

func pkgWithMerge(file string, merge map[string]any, arrays string) []*manifest.Package {
	return []*manifest.Package{{Name: "pi", Steps: []manifest.Step{
		manifest.JSONMergeStep{File: file, Merge: merge, Arrays: arrays},
	}}}
}

func TestD1FlagsAppendSuperset(t *testing.T) {
	ctx := ctxT(t)
	f := filepath.Join(t.TempDir(), "settings.json")
	writeFile(t, f, `{"packages":["npm:a","npm:a@1.0.0","npm:b"]}`)
	pkgs := pkgWithMerge(f, map[string]any{"packages": []any{"npm:a", "npm:b"}}, "")
	got := CheckExtraArray(ctx, pkgs)
	if len(got) != 1 || got[0].Severity != Warn {
		t.Fatalf("want 1 warn, got %+v", got)
	}
}

func TestD1ReplaceExtrasAreError(t *testing.T) {
	ctx := ctxT(t)
	f := filepath.Join(t.TempDir(), "settings.json")
	writeFile(t, f, `{"packages":["npm:a","npm:x"]}`)
	pkgs := pkgWithMerge(f, map[string]any{"packages": []any{"npm:a"}}, "replace")
	got := CheckExtraArray(ctx, pkgs)
	if len(got) != 1 || got[0].Severity != Error {
		t.Fatalf("want 1 error, got %+v", got)
	}
}

func TestD1CleanNoFindings(t *testing.T) {
	ctx := ctxT(t)
	f := filepath.Join(t.TempDir(), "settings.json")
	writeFile(t, f, `{"packages":["npm:a"]}`)
	if got := CheckExtraArray(ctx, pkgWithMerge(f, map[string]any{"packages": []any{"npm:a"}}, "replace")); len(got) != 0 {
		t.Fatalf("want none, got %+v", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/doctor/ -run TestD1 -v`
Expected: FAIL — `undefined: CheckExtraArray`.

- [ ] **Step 3: Write minimal implementation**

```go
package doctor

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/schuettc/kempt/internal/jsonutil"
	"github.com/schuettc/kempt/internal/machine"
	"github.com/schuettc/kempt/internal/manifest"
)

func CheckExtraArray(ctx *machine.Context, pkgs []*manifest.Package) []Finding {
	var out []Finding
	for _, pkg := range pkgs {
		for _, step := range pkg.Steps {
			jm, ok := step.(manifest.JSONMergeStep)
			if !ok {
				continue
			}
			file := ctx.Expand(jm.File)
			b, err := os.ReadFile(file)
			if err != nil {
				continue // missing/unreadable is plan's concern
			}
			var live map[string]any
			if json.Unmarshal(b, &live) != nil {
				continue
			}
			for key, dv := range jm.Merge {
				desired, ok := toArray(dv)
				if !ok {
					continue
				}
				current, ok := live[key].([]any)
				if !ok {
					continue
				}
				extras := jsonutil.ArrayExtras(desired, current)
				if len(extras) == 0 {
					continue
				}
				sev := Warn
				rem := fmt.Sprintf("set arrays=\"replace\" on this json-merge and run `kempt apply`, or add them to the manifest")
				if jm.Arrays == "replace" {
					sev = Error
					rem = "run `kempt apply` to reconcile (arrays=replace should already remove these)"
				}
				out = append(out, Finding{
					Check:       "extra-array",
					Package:     pkg.Name,
					Severity:    sev,
					Detail:      fmt.Sprintf("%s .%s has %d undeclared entr(y/ies): %v", file, key, len(extras), extras),
					Remediation: rem,
				})
			}
		}
	}
	return out
}

// toArray coerces a TOML/JSON-decoded value to []any if it is array-shaped.
func toArray(v any) ([]any, bool) {
	a, ok := v.([]any)
	return a, ok
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/doctor/ -run TestD1 -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/doctor/d1_extra_array.go internal/doctor/d1_extra_array_test.go
git commit -m "feat(doctor): D1 extra-array drift check"
```

---

### Task 6: D2 — duplicate identity within a managed array

**Files:**
- Create: `internal/doctor/d2_dup_identity.go`
- Test: `internal/doctor/d2_dup_identity_test.go`

**Interfaces:**
- Consumes: `jsonutil.SpecIdentity`; same `JSONMergeStep` iteration as D1.
- Produces: `func CheckDupIdentity(ctx *machine.Context, pkgs []*manifest.Package) []Finding` — for each array the manifest manages, group live string entries by `SpecIdentity`; emit one `warn` per identity appearing more than once.

- [ ] **Step 1: Write the failing test**

```go
package doctor

import (
	"path/filepath"
	"testing"
)

func TestD2FlagsDuplicateIdentity(t *testing.T) {
	ctx := ctxT(t)
	f := filepath.Join(t.TempDir(), "settings.json")
	writeFile(t, f, `{"packages":["npm:pi-quiet","npm:pi-quiet@0.2.0","npm:solo"]}`)
	pkgs := pkgWithMerge(f, map[string]any{"packages": []any{"npm:pi-quiet", "npm:solo"}}, "")
	got := CheckDupIdentity(ctx, pkgs)
	if len(got) != 1 || got[0].Severity != Warn {
		t.Fatalf("want 1 warn for pi-quiet, got %+v", got)
	}
}

func TestD2NoDuplicates(t *testing.T) {
	ctx := ctxT(t)
	f := filepath.Join(t.TempDir(), "settings.json")
	writeFile(t, f, `{"packages":["npm:a","npm:b"]}`)
	if got := CheckDupIdentity(ctx, pkgWithMerge(f, map[string]any{"packages": []any{"npm:a", "npm:b"}}, "")); len(got) != 0 {
		t.Fatalf("want none, got %+v", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/doctor/ -run TestD2 -v`
Expected: FAIL — `undefined: CheckDupIdentity`.

- [ ] **Step 3: Write minimal implementation**

```go
package doctor

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/schuettc/kempt/internal/jsonutil"
	"github.com/schuettc/kempt/internal/machine"
	"github.com/schuettc/kempt/internal/manifest"
)

func CheckDupIdentity(ctx *machine.Context, pkgs []*manifest.Package) []Finding {
	var out []Finding
	for _, pkg := range pkgs {
		for _, step := range pkg.Steps {
			jm, ok := step.(manifest.JSONMergeStep)
			if !ok {
				continue
			}
			b, err := os.ReadFile(ctx.Expand(jm.File))
			if err != nil {
				continue
			}
			var live map[string]any
			if json.Unmarshal(b, &live) != nil {
				continue
			}
			for key := range jm.Merge {
				current, ok := live[key].([]any)
				if !ok {
					continue
				}
				byID := map[string][]string{}
				for _, e := range current {
					s, ok := e.(string)
					if !ok {
						continue
					}
					id := jsonutil.SpecIdentity(s)
					byID[id] = append(byID[id], s)
				}
				var ids []string
				for id, variants := range byID {
					if len(variants) > 1 {
						ids = append(ids, id)
					}
				}
				sort.Strings(ids)
				for _, id := range ids {
					out = append(out, Finding{
						Check:       "dup-identity",
						Package:     pkg.Name,
						Severity:    Warn,
						Detail:      fmt.Sprintf("%s .%s lists %q %d times: %v", ctx.Expand(jm.File), id, len(byID[id]), byID[id]),
						Remediation: "run `kempt apply` (an arrays=replace merge collapses these) or dedupe the file",
					})
				}
			}
		}
	}
	return out
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/doctor/ -run TestD2 -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/doctor/d2_dup_identity.go internal/doctor/d2_dup_identity_test.go
git commit -m "feat(doctor): D2 duplicate-identity check"
```

---

### Task 7: `[doctor]` manifest config + D3 orphan-install check

**Files:**
- Modify: `internal/manifest/types.go` (add `Doctor *DoctorConfig` to `Manifest`, `type DoctorConfig`)
- Modify: `internal/manifest/parse.go` (decode `[doctor]`)
- Modify: `internal/schema/schema.go` or the schema source (add `doctor` object: `checkNpmOrphans` bool, `ignore` string array)
- Create: `internal/doctor/d3_orphans.go`
- Test: `internal/doctor/d3_orphans_test.go`
- Test: `internal/manifest/parse_test.go` (add a `[doctor]` decode case)

**Interfaces:**
- Produces:
  - `type DoctorConfig struct { CheckNpmOrphans bool; Ignore []string }` on `Manifest.Doctor`.
  - `func CheckOrphans(ctx *machine.Context, pkgs []*manifest.Package, cfg manifest.DoctorConfig) []Finding` — uses `inventory.Pi` (always) and `inventory.Npm` (only when `cfg.CheckNpmOrphans`); reports installed identities not declared by any selected `install` step's `Pi`/`Npm` list; suppresses any whose identity or full spec matches an `Ignore` entry (exact or `glob:` prefix). pi orphans → `warn`; npm orphans → `info`.

- [ ] **Step 1: Write the failing test**

```go
package doctor

import (
	"testing"

	"github.com/schuettc/kempt/internal/machine"
	"github.com/schuettc/kempt/internal/manifest"
	"github.com/schuettc/kempt/internal/run"
)

func ctxRunner(t *testing.T, fake *run.FakeRunner) *machine.Context {
	c, _ := machine.New(t.TempDir(), fake)
	c.Home = t.TempDir()
	return c
}

func TestD3FlagsPiOrphan(t *testing.T) {
	ctx := ctxRunner(t, &run.FakeRunner{Responses: map[string]run.Response{
		"pi list": {Stdout: "npm:declared@1.0.0\nnpm:orphan@2.0.0\n"},
	}})
	pkgs := []*manifest.Package{{Name: "pi", Steps: []manifest.Step{
		manifest.InstallStep{Pi: []string{"npm:declared"}},
	}}}
	got := CheckOrphans(ctx, pkgs, manifest.DoctorConfig{})
	if len(got) != 1 || got[0].Severity != Warn || got[0].Check != "orphan" {
		t.Fatalf("want 1 warn orphan, got %+v", got)
	}
}

func TestD3IgnoreSuppresses(t *testing.T) {
	ctx := ctxRunner(t, &run.FakeRunner{Responses: map[string]run.Response{
		"pi list": {Stdout: "npm:orphan@2.0.0\n"},
	}})
	pkgs := []*manifest.Package{{Name: "pi", Steps: []manifest.Step{manifest.InstallStep{Pi: []string{}}}}}
	got := CheckOrphans(ctx, pkgs, manifest.DoctorConfig{Ignore: []string{"npm:orphan"}})
	if len(got) != 0 {
		t.Fatalf("ignore should suppress, got %+v", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/doctor/ -run TestD3 -v`
Expected: FAIL — `undefined: CheckOrphans` (and `manifest.DoctorConfig`).

- [ ] **Step 3a: Add the manifest config**

In `internal/manifest/types.go`:

```go
type DoctorConfig struct {
	CheckNpmOrphans bool     `toml:"checkNpmOrphans"`
	Ignore          []string `toml:"ignore"`
}
```

Add `Doctor *DoctorConfig` to `Manifest` and decode `[doctor]` in `parse.go` (follow how an existing top-level table is decoded). In the schema source add a `doctor` object with `checkNpmOrphans` (boolean) and `ignore` (array of strings).

- [ ] **Step 3b: Implement the check**

```go
package doctor

import (
	"fmt"
	"path"
	"sort"

	"github.com/schuettc/kempt/internal/inventory"
	"github.com/schuettc/kempt/internal/jsonutil"
	"github.com/schuettc/kempt/internal/machine"
	"github.com/schuettc/kempt/internal/manifest"
)

func CheckOrphans(ctx *machine.Context, pkgs []*manifest.Package, cfg manifest.DoctorConfig) []Finding {
	declaredPi, declaredNpm := map[string]bool{}, map[string]bool{}
	for _, pkg := range pkgs {
		for _, step := range pkg.Steps {
			is, ok := step.(manifest.InstallStep)
			if !ok {
				continue
			}
			for _, s := range is.Pi {
				declaredPi[jsonutil.SpecIdentity(s)] = true
			}
			for _, s := range is.Npm {
				declaredNpm[jsonutil.SpecIdentity("npm:"+s)] = true
			}
		}
	}
	ignored := func(id string) bool {
		for _, ig := range cfg.Ignore {
			if ig == id || ig == "npm:"+id {
				continue2 := false
				_ = continue2
				return true
			}
			if len(ig) > 5 && ig[:5] == "glob:" {
				if ok, _ := path.Match(ig[5:], id); ok {
					return true
				}
			}
		}
		return false
	}
	var out []Finding
	piInv, err := inventory.Pi(ctx)
	if err == nil {
		var ids []string
		for spec := range piInv {
			ids = append(ids, spec)
		}
		sort.Strings(ids)
		for _, spec := range ids {
			id := jsonutil.SpecIdentity(spec)
			if declaredPi[id] || ignored(id) {
				continue
			}
			out = append(out, Finding{
				Check: "orphan", Package: "pi", Severity: Warn,
				Detail:      fmt.Sprintf("%s is installed but not declared in the manifest", spec),
				Remediation: "`kempt adopt` it into a package to keep it, or remove it (`pi remove " + spec + "`)",
			})
		}
	}
	if cfg.CheckNpmOrphans {
		npmInv, err := inventory.Npm(ctx)
		if err == nil {
			var names []string
			for name := range npmInv {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				id := jsonutil.SpecIdentity("npm:" + name)
				if declaredNpm[id] || ignored(id) {
					continue
				}
				out = append(out, Finding{
					Check: "orphan", Package: "npm", Severity: Info,
					Detail:      fmt.Sprintf("global npm %q is installed but not declared", name),
					Remediation: "add it to an install.npm list, add to [doctor].ignore, or uninstall it",
				})
			}
		}
	}
	return out
}
```

(Simplify the `ignored` helper during implementation — the sketch shows intent: exact match on `id` or `npm:id`, plus `glob:` patterns.)

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/doctor/ -run TestD3 -v && go test ./internal/manifest/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/manifest/ internal/schema/ internal/doctor/d3_orphans.go internal/doctor/d3_orphans_test.go
git commit -m "feat(doctor): [doctor] config + D3 orphan-install check"
```

---

### Task 8: D4 — broken/foreign managed symlinks

**Files:**
- Create: `internal/doctor/d4_symlinks.go`
- Test: `internal/doctor/d4_symlinks_test.go`

**Interfaces:**
- Consumes: manifest `SymlinkStep` (`From string`, `To string`); `machine.Context.Expand`, `.RepoDir`.
- Produces: `func CheckSymlinks(ctx *machine.Context, pkgs []*manifest.Package) []Finding` — for each `symlink` step, `os.Lstat` the expanded `To`: **broken** (symlink whose target does not exist) → `error`; **foreign** (a real file/dir, not a symlink) → `warn`; **mispointed** (symlink to a target other than the repo `From`) → `warn`; correct or absent → none.

- [ ] **Step 1: Write the failing test**

```go
package doctor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/schuettc/kempt/internal/manifest"
)

func TestD4ForeignFile(t *testing.T) {
	ctx := ctxT(t)
	to := filepath.Join(t.TempDir(), "link")
	writeFile(t, to, "real file where a symlink is expected")
	pkgs := []*manifest.Package{{Name: "p", Steps: []manifest.Step{
		manifest.SymlinkStep{From: "src", To: to},
	}}}
	got := CheckSymlinks(ctx, pkgs)
	if len(got) != 1 || got[0].Severity != Warn {
		t.Fatalf("want 1 warn (foreign), got %+v", got)
	}
}

func TestD4BrokenLink(t *testing.T) {
	ctx := ctxT(t)
	to := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(filepath.Join(t.TempDir(), "does-not-exist"), to); err != nil {
		t.Fatal(err)
	}
	got := CheckSymlinks(ctx, []*manifest.Package{{Name: "p", Steps: []manifest.Step{
		manifest.SymlinkStep{From: "src", To: to},
	}}})
	if len(got) != 1 || got[0].Severity != Error {
		t.Fatalf("want 1 error (broken), got %+v", got)
	}
}

func TestD4AbsentNoFinding(t *testing.T) {
	ctx := ctxT(t)
	to := filepath.Join(t.TempDir(), "nope")
	if got := CheckSymlinks(ctx, []*manifest.Package{{Name: "p", Steps: []manifest.Step{
		manifest.SymlinkStep{From: "src", To: to},
	}}}); len(got) != 0 {
		t.Fatalf("absent target is plan's job, want none, got %+v", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/doctor/ -run TestD4 -v`
Expected: FAIL — `undefined: CheckSymlinks`.

- [ ] **Step 3: Write minimal implementation**

```go
package doctor

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/schuettc/kempt/internal/machine"
	"github.com/schuettc/kempt/internal/manifest"
)

func CheckSymlinks(ctx *machine.Context, pkgs []*manifest.Package) []Finding {
	var out []Finding
	for _, pkg := range pkgs {
		for _, step := range pkg.Steps {
			sl, ok := step.(manifest.SymlinkStep)
			if !ok {
				continue
			}
			to := ctx.Expand(sl.To)
			fi, err := os.Lstat(to)
			if err != nil {
				continue // absent → plan will create it
			}
			want := filepath.Join(ctx.RepoDir, sl.From)
			if fi.Mode()&os.ModeSymlink == 0 {
				out = append(out, Finding{
					Check: "symlink", Package: pkg.Name, Severity: Warn,
					Detail:      fmt.Sprintf("%s is a real %s where a managed symlink is expected", to, kindOf(fi)),
					Remediation: "run `kempt apply` (it will back up and relink)",
				})
				continue
			}
			target, _ := os.Readlink(to)
			if _, err := os.Stat(to); err != nil {
				out = append(out, Finding{
					Check: "symlink", Package: pkg.Name, Severity: Error,
					Detail:      fmt.Sprintf("%s is a broken symlink → %s", to, target),
					Remediation: "run `kempt apply` to relink, or remove the dangling link",
				})
				continue
			}
			if target != want {
				out = append(out, Finding{
					Check: "symlink", Package: pkg.Name, Severity: Warn,
					Detail:      fmt.Sprintf("%s points to %s, expected %s", to, target, want),
					Remediation: "run `kempt apply` to repoint it",
				})
			}
		}
	}
	return out
}

func kindOf(fi os.FileInfo) string {
	if fi.IsDir() {
		return "directory"
	}
	return "file"
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/doctor/ -run TestD4 -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/doctor/d4_symlinks.go internal/doctor/d4_symlinks_test.go
git commit -m "feat(doctor): D4 broken/foreign managed symlink check"
```

---

### Task 9: D5 — plan/verify rollup

**Files:**
- Create: `internal/doctor/d5_rollup.go`
- Test: `internal/doctor/d5_rollup_test.go`

**Interfaces:**
- Consumes: `engine.BuildPlan`, `engine.Op` (`OpChange`, `OpBlocked`), `engine.HandlerFor("verify")`, `engine.OnlySkip`, `engine.StepOnly`.
- Produces: `func CheckRollup(ctx *machine.Context, pkgs []*manifest.Package) []Finding` — build the plan, count `OpChange` (→ one `info` "N pending changes; run kempt apply" when >0) and `OpBlocked` (→ one `error` per blocked step with its Detail); run verify steps and emit one `error` per `OpBlocked` verify result.

- [ ] **Step 1: Write the failing test**

```go
package doctor

import (
	"testing"

	"github.com/schuettc/kempt/internal/machine"
	"github.com/schuettc/kempt/internal/manifest"
	"github.com/schuettc/kempt/internal/run"
)

func TestD5PendingChangeIsInfo(t *testing.T) {
	// A symlink step whose target is absent → plan reports OpChange.
	c, _ := machine.New(t.TempDir(), &run.FakeRunner{})
	c.Home = t.TempDir()
	pkgs := []*manifest.Package{{Name: "p", Steps: []manifest.Step{
		manifest.SymlinkStep{From: "src", To: c.Home + "/absent-link"},
	}}}
	got := CheckRollup(c, pkgs)
	var infos int
	for _, f := range got {
		if f.Severity == Info {
			infos++
		}
	}
	if infos == 0 {
		t.Fatalf("want an info rollup for pending changes, got %+v", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/doctor/ -run TestD5 -v`
Expected: FAIL — `undefined: CheckRollup`.

- [ ] **Step 3: Write minimal implementation**

```go
package doctor

import (
	"fmt"

	"github.com/schuettc/kempt/internal/engine"
	"github.com/schuettc/kempt/internal/machine"
	"github.com/schuettc/kempt/internal/manifest"
)

func CheckRollup(ctx *machine.Context, pkgs []*manifest.Package) []Finding {
	var out []Finding
	plan, err := engine.BuildPlan(ctx, pkgs)
	if err != nil {
		return []Finding{{Check: "rollup", Severity: Error, Detail: "plan failed: " + err.Error(), Remediation: "run `kempt plan` to see the error"}}
	}
	pending := 0
	for _, pp := range plan.Packages {
		for _, sr := range pp.Steps {
			switch sr.Delta.Op {
			case engine.OpChange:
				pending++
			case engine.OpBlocked:
				out = append(out, Finding{
					Check: "rollup", Package: pp.Name, Severity: Error,
					Detail:      "blocked: " + sr.Delta.Detail,
					Remediation: "resolve the blocker; see `kempt plan`",
				})
			}
		}
	}
	if pending > 0 {
		out = append(out, Finding{
			Check: "rollup", Severity: Info,
			Detail:      fmt.Sprintf("%d pending change(s) — machine is behind the manifest", pending),
			Remediation: "run `kempt apply` to converge",
		})
	}

	if h, ok := engine.HandlerFor("verify"); ok {
		for _, pkg := range pkgs {
			if _, skip := engine.OnlySkip(ctx, pkg.Only); skip {
				continue
			}
			for _, step := range pkg.Steps {
				if step.Kind() != "verify" {
					continue
				}
				if _, skip := engine.OnlySkip(ctx, engine.StepOnly(step)); skip {
					continue
				}
				d, err := h.Inspect(ctx, step)
				if err != nil || d.Op == engine.OpBlocked {
					detail := d.Detail
					if err != nil {
						detail = err.Error()
					}
					out = append(out, Finding{
						Check: "rollup", Package: pkg.Name, Severity: Error,
						Detail:      "verify failed: " + detail,
						Remediation: "run `kempt verify` and fix the failing check",
					})
				}
			}
		}
	}
	return out
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/doctor/ -run TestD5 -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/doctor/d5_rollup.go internal/doctor/d5_rollup_test.go
git commit -m "feat(doctor): D5 plan/verify rollup"
```

---

### Task 10: `doctor.Run` aggregator

**Files:**
- Create: `internal/doctor/run.go`
- Test: `internal/doctor/run_test.go`

**Interfaces:**
- Consumes: all five `Check*` functions; `manifest.DoctorConfig`.
- Produces: `func Run(ctx *machine.Context, pkgs []*manifest.Package, cfg manifest.DoctorConfig) Report` — calls D1–D5 in order, concatenates findings into a `Report`.

- [ ] **Step 1: Write the failing test**

```go
package doctor

import (
	"path/filepath"
	"testing"

	"github.com/schuettc/kempt/internal/manifest"
	"github.com/schuettc/kempt/internal/run"
)

func TestRunAggregates(t *testing.T) {
	c := ctxRunner(t, &run.FakeRunner{Responses: map[string]run.Response{"pi list": {Stdout: ""}}})
	f := filepath.Join(t.TempDir(), "settings.json")
	writeFile(t, f, `{"packages":["npm:a","npm:a@1.0.0"]}`)
	pkgs := pkgWithMerge(f, map[string]any{"packages": []any{"npm:a"}}, "")
	rep := Run(c, pkgs, manifest.DoctorConfig{})
	// D1 (extra) + D2 (dup) both fire on the same file.
	if len(rep.Findings) < 2 {
		t.Fatalf("want >=2 findings from D1+D2, got %+v", rep.Findings)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/doctor/ -run TestRunAggregates -v`
Expected: FAIL — `undefined: Run`.

- [ ] **Step 3: Write minimal implementation**

```go
package doctor

import (
	"github.com/schuettc/kempt/internal/machine"
	"github.com/schuettc/kempt/internal/manifest"
)

// Run executes every read-only check and returns their combined Report.
func Run(ctx *machine.Context, pkgs []*manifest.Package, cfg manifest.DoctorConfig) Report {
	var f []Finding
	f = append(f, CheckExtraArray(ctx, pkgs)...)
	f = append(f, CheckDupIdentity(ctx, pkgs)...)
	f = append(f, CheckOrphans(ctx, pkgs, cfg)...)
	f = append(f, CheckSymlinks(ctx, pkgs)...)
	f = append(f, CheckRollup(ctx, pkgs)...)
	return Report{Findings: f}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/doctor/ -v`
Expected: PASS (whole package).

- [ ] **Step 5: Commit**

```bash
git add internal/doctor/run.go internal/doctor/run_test.go
git commit -m "feat(doctor): Run aggregator composing D1-D5"
```

---

### Task 11: `kempt doctor` CLI command

**Files:**
- Create: `internal/cli/doctor.go`
- Modify: `internal/cli/commands_test.go` (add `"doctor"` to the required-names list)
- Test: `internal/cli/doctor_test.go`

**Interfaces:**
- Consumes: `doctor.Run`, `doctor.Report`; the selection helpers `loadState`, `resolveManifest`, `resolveSelection`, `splitPackages`, `loadManifestSource`, `newContext`, `manifest.Parse`, `manifest.Validate`, `engine.Select` (mirror `runVerify`).
- Produces: a registered `doctor` command; exit `1` when `report.Failed(strict)`.

- [ ] **Step 1: Write the failing test**

```go
package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestDoctorRegistered(t *testing.T) {
	var out, errw bytes.Buffer
	if code := Dispatch([]string{"commands", "--json"}, &out, &errw); code != 0 {
		t.Fatalf("commands exit=%d", code)
	}
	if !strings.Contains(out.String(), "\"doctor\"") {
		t.Fatalf("doctor not in commands index: %s", out.String())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/ -run TestDoctorRegistered -v`
Expected: FAIL — doctor not registered.

- [ ] **Step 3: Write minimal implementation**

```go
package cli

import (
	"flag"
	"io"
	"os"

	"github.com/schuettc/kempt/internal/doctor"
	"github.com/schuettc/kempt/internal/engine"
	_ "github.com/schuettc/kempt/internal/engine/handlers"
	"github.com/schuettc/kempt/internal/manifest"
)

func init() {
	Register(Command{
		Name:     "doctor",
		Summary:  "diagnose machine-vs-manifest drift (read-only)",
		Synopsis: "doctor [flags]",
		Help:     "Reports drift the plan/verify commands miss: undeclared live array entries, duplicate package identities, orphaned installs, broken/foreign managed symlinks, and a plan/verify rollup. Read-only. Exits non-zero when findings reach the failing threshold (error by default; warn under -strict).",
		NewFlags: func() *flag.FlagSet { fs, _ := newDoctorFlags(); return fs },
		Run:      runDoctor,
	})
}

type doctorFlags struct {
	manifest *string
	profile  *string
	packages *string
	json     *bool
	strict   *bool
}

func newDoctorFlags() (*flag.FlagSet, *doctorFlags) {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	v := &doctorFlags{
		manifest: fs.String("manifest", "", "path to manifest"),
		profile:  fs.String("profile", "", "profile to select"),
		packages: fs.String("packages", "", "comma-separated package names"),
		json:     fs.Bool("json", false, "emit machine-readable findings"),
		strict:   fs.Bool("strict", false, "treat warnings as failures for the exit code"),
	}
	return fs, v
}

func runDoctor(args []string, out, errw io.Writer) error {
	fs, v := newDoctorFlags()
	if err := ParseFlags(fs, args, out); err != nil {
		return err
	}
	st, existed, err := loadState()
	if err != nil {
		return err
	}
	manifestPath := resolveManifest(*v.manifest, st, existed)
	profile, packages := resolveSelection(*v.profile, splitPackages(*v.packages), st, existed)
	src, repoDir, name, err := loadManifestSource(manifestPath, os.Stdin)
	if err != nil {
		return UsageError{Msg: err.Error()}
	}
	m, findings := manifest.Parse(src)
	if m != nil {
		findings = append(findings, manifest.Validate(m)...)
	}
	if len(findings) > 0 {
		for _, f := range findings {
			fmtFprintf(errw, name, f)
		}
		return UsageError{Msg: "manifest has findings; run kempt lint"}
	}
	ctx, err := newContext(repoDir)
	if err != nil {
		return err
	}
	selected, err := engine.Select(m, profile, packages)
	if err != nil {
		return UsageError{Msg: err.Error()}
	}
	var cfg manifest.DoctorConfig
	if m.Doctor != nil {
		cfg = *m.Doctor
	}
	report := doctor.Run(ctx, selected, cfg)
	if *v.json {
		if err := report.WriteJSON(out); err != nil {
			return err
		}
	} else {
		report.WriteHuman(out)
	}
	if report.Failed(*v.strict) {
		return silentExitError{} // maps to exit 1 without extra output
	}
	return nil
}
```

Implement `fmtFprintf` inline as the same `fmt.Fprintf(errw, "%s: %s: %s\n", name, f.Path, f.Msg)` used by `runVerify` (or just inline that call). For the non-zero exit without a duplicate error message, check how `verify` returns `fmt.Errorf(...)` and how Dispatch prints it; if a bare error double-prints, add a minimal `silentExitError` type whose `Error()` returns `""` and confirm Dispatch maps non-nil error → exit 1. If the shared `tools` Dispatch prints the error text, prefer returning `fmt.Errorf("%d finding(s)", n)` for parity with verify's `"N verify check(s) failed"`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/cli/ -run 'TestDoctor|TestCommandsJSON' -v`
Expected: PASS. Then add `"doctor"` to the required-names slice in `commands_test.go` and re-run.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/doctor.go internal/cli/doctor_test.go internal/cli/commands_test.go
git commit -m "feat(cli): kempt doctor command"
```

---

### Task 12: L1 — lint the install↔settings string-for-string invariant

**Files:**
- Modify: `internal/manifest/validate.go`
- Test: `internal/manifest/validate_test.go`

**Interfaces:**
- Consumes: `Manifest`, `Package`, `InstallStep.Pi`, `JSONMergeStep` (`File`, `Merge`).
- Produces: additional `Finding`s from `Validate`: for each package that has both an `install` step with a non-empty `Pi` list AND a `json-merge` step whose `Merge["packages"]` is an array, assert the two are set-equal (same set of exact strings). On mismatch, emit a `Finding` naming the package and the first differing entry.

- [ ] **Step 1: Write the failing test**

```go
func TestValidateInstallSettingsPackagesMismatch(t *testing.T) {
	src := []byte(`
spec = 1
[packages.pi]
[[packages.pi.install]]
pi = ["npm:a", "npm:b"]
[[packages.pi.json-merge]]
file = "~/.pi/agent/settings.json"
merge = { packages = ["npm:a"] }
`)
	m, f := manifest.Parse(src)
	f = append(f, manifest.Validate(m)...)
	found := false
	for _, x := range f {
		if strings.Contains(x.Msg, "install") && strings.Contains(x.Msg, "packages") {
			found = true
		}
	}
	if !found {
		t.Fatalf("want a string-for-string mismatch finding, got %+v", f)
	}
}

func TestValidateInstallSettingsPackagesMatchOrderInsensitive(t *testing.T) {
	src := []byte(`
spec = 1
[packages.pi]
[[packages.pi.install]]
pi = ["npm:a", "npm:b"]
[[packages.pi.json-merge]]
file = "~/.pi/agent/settings.json"
merge = { packages = ["npm:b", "npm:a"] }
`)
	m, f := manifest.Parse(src)
	f = append(f, manifest.Validate(m)...)
	for _, x := range f {
		if strings.Contains(x.Msg, "string-for-string") {
			t.Fatalf("reordered lists are set-equal; no finding expected: %+v", f)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/manifest/ -run TestValidateInstallSettings -v`
Expected: FAIL — no such finding yet.

- [ ] **Step 3: Write minimal implementation**

In `validate.go`, add a pass over `m.Packages`: collect the package's `install.Pi` set and its `json-merge` `Merge["packages"]` string set (only when the merge `file` ends with the pi settings path — match `strings.HasSuffix(file, "/.pi/agent/settings.json")` or contains `pi/agent/settings.json`). Compare sets; on inequality append `Finding{Path: "packages." + name, Msg: "install pi list and settings-merge packages differ string-for-string: <first-diff>"}`. Use the existing per-kind index convention for `Path` if a step index is readily available; otherwise the package path is acceptable.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/manifest/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/manifest/validate.go internal/manifest/validate_test.go
git commit -m "feat(lint): enforce install-list == settings-merge packages string-for-string"
```

---

### Task 13: Docs — spec, README, help surfaces

**Files:**
- Modify: `docs/spec.md` (command list / `doctor` description + the `[doctor]` block)
- Modify: `README.md` (day-to-day command list row for `doctor`)
- Verify: `kempt help doctor` and `kempt man` render the `Help` text (no code change; they read the `Command.Help` field)

- [ ] **Step 1: Add the command to docs**

In `docs/spec.md`, add a `doctor` row/description to the command list and document the `[doctor]` config keys (`checkNpmOrphans`, `ignore`). In `README.md`, add a row to the day-to-day table: `| Diagnose drift (read-only) | `kempt doctor` |`.

- [ ] **Step 2: Verify help/man render**

Run: `go run ./cmd/kempt help doctor && go run ./cmd/kempt man | grep -A2 doctor`
Expected: the doctor summary/help text appears.

- [ ] **Step 3: Full suite green**

Run: `go test ./... && go vet ./...`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add docs/spec.md README.md
git commit -m "docs: document kempt doctor and [doctor] config"
```

---

## Self-Review

**Spec coverage:**
- D1 extra-array → Task 5. D2 dup identity → Task 6. D3 orphans (+ `[doctor]` config + ignore + npm opt-in) → Task 7. D4 symlinks → Task 8. D5 plan/verify rollup → Task 9. Aggregator → Task 10. Command (flags, exit codes, `--json`, `-strict`, selection parity with verify) → Task 11. L1 lint invariant → Task 12. Docs/help/man → Task 13. Shared helpers (identity, extras, inventories) → Tasks 1–3. All spec sections mapped.
- Report-only / remediation strings: every Finding constructor sets `Remediation`; no `--fix` anywhere. ✓
- Exit-code model (0/1/2, strict lowers threshold): Task 4 `Failed`, Task 11 mapping. ✓

**Placeholder scan:** Task 7's `ignored` helper sketch and Task 11's exit-error mapping are explicitly flagged as "simplify/confirm during implementation" with the exact behavior stated — not silent TODOs. Task 12 `Path` uses the package path with a note on the index convention. No "TBD"/"handle edge cases" left.

**Type consistency:** `Finding{Check,Package,Detail,Remediation string; Severity}` used identically across Tasks 4–11. Check function signature `func Check*(ctx *machine.Context, pkgs []*manifest.Package[, cfg]) []Finding` consistent; `Run` calls them with matching arity (D3 takes `cfg`, others don't — matches Task 10). `manifest.DoctorConfig{CheckNpmOrphans bool; Ignore []string}` defined in Task 7, consumed in Tasks 7/10/11. `inventory.Pi`/`inventory.Npm` defined in Task 3, used in Task 7. `jsonutil.SpecIdentity`/`ArrayExtras` defined Tasks 1–2, used Tasks 5–7.

## Execution Handoff

Plan complete and saved to `docs/plans/2026-09-21-kempt-doctor.md`. Two execution options:

1. **Subagent-Driven (recommended)** — a fresh subagent per task, review between tasks, fast iteration.
2. **Inline Execution** — execute tasks in this session with checkpoints for review.

Which approach?
