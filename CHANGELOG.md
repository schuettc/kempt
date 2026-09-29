# Changelog

## 0.5.7

- macOS binaries are signed with the Developer ID and notarized, so a copy downloaded in a browser runs instead of being quarantined by Gatekeeper. Releases are built through the family release actions (`schuettc/tools-actions`); the assets and `/dl` paths are unchanged.

## 0.5.6

- Built on tools-common v0.8.2: `kempt help <command>` and `<command> -h` show the command's summary line, and `kempt man` escapes quotes and leading dots correctly.
- Dependabot proposes each tools-common and tools-actions release as a PR.

## 0.5.5

- `update` always reports where the binary stands (`kempt X (current)` or `kempt updated A -> B`) and ends the roll step with `rolling: N checked, M rolled, K skipped`, so a run with nothing to do is visibly a no-op

## 0.5.4

- unpinned `git:` pi entries now roll: `update`/`upgrade` move them to the repo's default-branch head and `outdated` reports them (installed and latest are shown as short commit shas)

## 0.5.3

- service steps take `start-interval` (launchd `StartInterval`) for periodic jobs
