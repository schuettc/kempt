# Changelog

## 0.5.8

- Checked by the stricter family lint set (tools-actions v0.6.0); the few findings are fixed or carry a stated reason. No behaviour change.
- `just verify` and the pre-push hook now run exactly what CI runs, at the version CI pins.
- The stale `install.sh` copy is removed from the repo; `https://kempt.tools/install.sh` is the installer.

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
