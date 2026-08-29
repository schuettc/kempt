# kempt

kempt is declarative machine setup you can read before you run: a single manifest describes the software and files a machine should have, and kempt shows you exactly what it would change before it changes anything. The manifest is `kempt.toml`; its schema and primitives are specified in [docs/spec.md](docs/spec.md).

## Getting started

```sh
curl -fsSL https://kempt.tools/install.sh | sh
```

The installer downloads the latest release for your platform, verifies its
checksum, and drops the `kempt` binary in `~/.local/bin` (override with
`KEMPT_INSTALL_DIR`). To build from source instead:

```sh
go install ./cmd/kempt
```

Then point kempt at your config repo:

```sh
kempt init <repo-url>
```

`kempt init` clones the manifest repo (default `~/.config/kempt/repo`), walks an
interactive picker to choose a profile and refine the exact set of packages,
saves your selection, and applies it to converge the machine. Pass `-profile
<name> -yes` to init non-interactively, or `-dir` to clone somewhere other than
the default.

Once a selection is saved, the day-to-day commands operate on it without
repeating `-manifest`/`-profile`:

- **`kempt status`** — show the cached result of the last refresh.
- **`kempt refresh`** — fetch the repo and recompute pending changes; with
  auto-apply enabled it applies files-class changes only (never software).
- **`kempt update`** — pull the repo, self-update the binary, and converge.
- **`kempt adopt <pkg>`** / **`kempt drop <pkg>`** — add or remove a package
  (and its needs) in the saved selection.
- **`kempt config auto-apply-files [true|false]`** — get or set whether refresh
  auto-applies files-class changes.

## Commands

kempt has the following working commands:

- **`kempt lint`** — validate a manifest without touching the machine.
  ```sh
  kempt lint kempt.toml
  ```
- **`kempt plan`** — inspect the machine and print the deltas apply would make.
  ```sh
  kempt plan -manifest kempt.toml
  ```
- **`kempt apply`** — converge the machine to the manifest (prompts unless `-yes`).
  ```sh
  kempt apply -manifest kempt.toml -yes
  ```
- **`kempt verify`** — run the manifest's read-only verify checks.
  ```sh
  kempt verify -manifest kempt.toml
  ```
- **`kempt version`** — print the kempt version.
  ```sh
  kempt version
  ```
- **`kempt schema`** — print the JSON Schema for `kempt.toml` (point your
  editor at it for completion and validation).
  ```sh
  kempt schema > kempt.schema.json
  ```
- **`kempt new`** — scaffold a new kempt config repo in the given directory
  (defaults to the current one; refuses to overwrite an existing `kempt.toml`).
  ```sh
  kempt new my-config
  ```
- **`kempt dump`** — inspect the current machine and suggest a manifest
  (read-only; never touches anything). Pass `-repo` to detect dotfile symlinks.
  ```sh
  kempt dump > kempt.toml
  ```

`plan`, `apply`, and `verify` accept `-profile` and `-packages` to narrow the
selection. `plan` also accepts `-os` and `-arch` to dry-plan for another
platform (e.g. `kempt plan -manifest kempt.toml -os linux -arch arm64`) without
inspecting the local machine's real OS/arch.

## Coming next

Dotfiles-as-reference-config and the [kempt.tools](https://kempt.tools) site
land next.
