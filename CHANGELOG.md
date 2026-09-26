# Changelog

## 0.5.5

- `update` always reports where the binary stands (`kempt X (current)` or `kempt updated A -> B`) and ends the roll step with `rolling: N checked, M rolled, K skipped`, so a run with nothing to do is visibly a no-op

## 0.5.4

- unpinned `git:` pi entries now roll: `update`/`upgrade` move them to the repo's default-branch head and `outdated` reports them (installed and latest are shown as short commit shas)

## 0.5.3

- service steps take `start-interval` (launchd `StartInterval`) for periodic jobs
