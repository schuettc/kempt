# kempt

kempt is declarative machine setup you can read before you run: a single manifest describes the software and files a machine should have, and kempt shows you exactly what it would change before it changes anything. The manifest is `kempt.toml`; its schema and primitives are specified in [docs/spec.md](docs/spec.md).

## Install from source

```sh
go install ./cmd/kempt
```

## Getting started

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

`plan`, `apply`, and `verify` accept `-profile` and `-packages` to narrow the selection.

## Coming next

Dotfiles-as-reference-config and the [kempt.tools](https://kempt.tools) site land next.
