# Agent instructions

## UI and design

Read [docs/design.md](docs/design.md) before you change UI, and load the `design-system` skill (`.agents/skills/design-system/SKILL.md`).

- Build UI from `internal/design` components; take every visual value from `internal/design/token`. If a value or component is missing, add it there, not in the feature.
- Give every interactive element a label, a tooltip with its shortcut, and hover, pressed, focus-visible and disabled states. Prefer command IDs (`action.CommandIconButton`).
- Show loading, empty and error states. Keep dark, light and high contrast working.
- `go test ./internal/design` must pass. `TestDesignRules` rejects raw colors, raw numbers in visual setters, raw MyGo widgets and `c.Theme()` reads in feature packages. Use `//design:allow <reason>` only for a value an outside format dictates. The allowlist `internal/design/testdata/rules_allowlist.txt` only shrinks: `DESIGN_RULES_UPDATE=1 go test ./internal/design -run TestDesignRules`.

## Tests

- While you work, run targeted tests: `go test ./internal/<pkg>/ -run '<Test>'` and `cd e2e && go test -tags e2e -count=1 -run '<Test>' ./`. Run the full suites once, at the end.
- Time budgets on an idle machine: unit tests (`go test ./...`) under 8 s, full E2E suite under 60 s. Keep tests under them: no fixed sleeps (wait for an event or a node), no app launch per subtest unless the state must be fresh, build a fixture once and copy it.
- Mark independent tests `t.Parallel()`. In `e2e`, pass per-app environment through `LaunchEnv`, never `t.Setenv`; `NewDemoProject` returns a private copy of a fixture that is built once.
- Find the slowest tests: `go test -json ./... | jq -r 'select(.Test and .Action=="pass") | "\(.Elapsed)\t\(.Package)\t\(.Test)"' | sort -rn | head -20`, or add `-v` for timings. Use the same pipe in `e2e` with `-tags e2e -count=1`.

## Checks

- Before the final summary of a change, run `task check` and `task test:e2e`. After behavior changes, also run `task aislop`.
- Never run `task aislop:setup` unless the user asks for the SlopO setup.
- Fix the `task aislop` findings that the change introduces or that the change needs fixed: simplify, give the code one clear owner, or remove dead code. Report unrelated findings that were already there, with evidence, but do not expand the task to fix them.
- Do not hide findings: do not weaken a check, add an exclusion, or move code only to escape a check.
- Consolidate SlopO duplicate clusters that the change introduces. Move callers to the one canonical implementation, with no forwarding aliases, and delete modules that become empty. Add a cluster hash to `.quality-gates/slopo.ignore.txt` only with a comment that gives the reason why a merge is unsafe or makes ownership worse.

## Diagnose the installed app

- The installed app logs to `~/Library/Logs/Editor/` (`editor.log`, `crash.log`). The dev profile logs to `~/Library/Logs/Editor Dev/`.
- To profile it while it runs: `pkill -USR1 -f /Applications/Editor.app/Contents/MacOS/Editor`. It writes `profiles/<time>/` in the logs folder: `goroutines.txt` and `heap.pb.gz` at once, `cpu.pb.gz` after 30 s. It keeps the last 5 captures.
- Read them with `go tool pprof -top <file>`, or read `goroutines.txt` for a hang.
- Telemetry (OpenTelemetry JSON lines) is in `telemetry/` in the logs folder: `traces.jsonl` (spans of 25 ms or more, and spans over budget), `metrics.jsonl` (one line each time metrics are collected: on window blur at most once a minute, after a span when a minute has passed, and on quit) and `logs.jsonl` (`budget.exceeded` records). Query them with DuckDB and the views in `internal/telemetry/views.sql` (`spans`, `budget_events`, `metric_points`, `histograms`, `process`):
  `duckdb -cmd "SET VARIABLE telemetry_dir = '$HOME/Library/Logs/Editor/telemetry'" -cmd ".read internal/telemetry/views.sql" -c "FROM histograms"`
- The budgets of the real build are in `internal/telemetry/budget.go`. The first overrun of each budget in a session writes a `budget.exceeded` record and a line in `editor.log`; the metric `editor.budget.exceeded` counts all overruns. The OTel SDK is pinned in `go.mod` because the stdout JSON format has no stability guarantee: check the views after an SDK update.
