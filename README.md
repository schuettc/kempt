# kempt

kempt is declarative machine setup you can read before you run: a single manifest describes the software and files a machine should have, and kempt shows you exactly what it would change before it changes anything. The manifest is `kempt.toml`; its schema and primitives are specified in [docs/spec.md](docs/spec.md).

## Install from source

```sh
go install ./cmd/kempt
```

## Commands

kempt has five working commands in this phase:

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

`kempt init` (scaffold a manifest), `kempt update` (refresh pinned versions), and the interactive package picker land in the next phase.
