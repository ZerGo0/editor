# Editor plan

A macOS code editor in Go, built with [MyGo](https://github.com/egoist/mygo) native UI.

Module path: `github.com/ZerGo0/editor`

## Decisions

| Topic | Decision |
|---|---|
| GUI | MyGo native UI (`ui`, `ui/editor`). Pure Go, no cgo. |
| Platform | macOS only for version 1. |
| Editor widget | MyGo `ui/editor`: rope buffer, multi-cursor, folding, find and replace, diff view, gutter changes. |
| Version 1 scope | Multi-cursor, split views. No plugins. |
| Syntax | MyGo's line-based lexer (38 languages). Add an `editor.Tokenizer` for each missing language. No LSP, no tree-sitter. |
| Large files | Full editing of files of 1 GB and more. Above about 512 MB, an own text widget with a piece table over the file, apart from MyGo's buffer (see [research](research/large-files.md)). |
| Keymap | Zed / VS Code defaults (MyGo editor defaults). |
| Git | All JetBrains Git features (see [research](research/git-jetbrains.md)). Use the git CLI. |
| Git hosting | Features of VS Code's GitHub Pull Requests extension, through the local `gh` CLI user when available (see [research](research/github-prs.md)). |
| Search Everywhere | Tabs: Files, Actions, Text, Symbols. Custom Go search, no fff (see [research](research/search-everywhere.md)). |
| Resources | Low memory and CPU. No frames while idle. |
| Tool windows | Local history (all saves, not only commits), Problems/TODO (from lexer comments). No terminal, no run tasks. |
| Settings | Settings UI and raw JSON file editing, as in VS Code. Same for the keymap. |
| Windows and tabs | Preview tabs, session restore (project, tabs, splits, cursors), one window for each project, recent projects. |
| Shipping | Local build only. No signing, no auto-update. |
| Shell command | `editor` command opens folders and files, and hands them to the running app. |
| Look | Native macOS: MyGo default theme, system accent color, sidebar. |
| Build mode | Claude writes the code milestone by milestone. The user reviews. |
| Target | The complete feature set. No reduced first release. |

## Constraints

- [See](https://github.com/wibus-wee/see) is a MyGo code reader with no license. Read it for ideas only. Do not copy its code.

## Development

- Icons: add a line `<name> <HugeiconsName> [filled]` to `internal/icons/icons.txt`, run `task icons`, then use `icons.<Name>`. Names come from hugeicons.com (free set).
- Checks: `task check` (vet, MyGo vet, gofmt, unit tests) and `task aislop`.
- E2E: `task test:e2e` builds the app with the `e2e` tag and drives hidden windows through the MyGo automation socket (`e2e/harness_test.go`). Native dialogs and context menus need unit or `ui.Tester` coverage.

## Research results

- **Large files** ([report](research/large-files.md)): MyGo's buffer loads 1 GB in 1.16 s, but peaks at 2.2 GB of memory. Find copies the whole text. `Buffer` is concrete, so no custom buffer can be plugged in. Proposed tiers:
  - Up to 20 MB: MyGo editor as is.
  - 20 to 512 MB: MyGo editor with highlighting, wrap, Git gutter and replace-all off, plus a streaming find.
  - Above 512 MB, 1/8 of RAM or 5M lines: a separate large-file mode with a pread block cache, a sparse line index built in the background, a piece table and a streaming search worker.
- **Search** ([report](research/search-everywhere.md)): Build in Go. Use the fzf matcher algorithm, `git ls-files` and fastwalk to list files, and `mygo.WatchFiles` (FSEvents) to see changes. Text search uses a parallel Go scan. fff has no Symbols or Actions tab, reads only files of 10 MB or less by default, and needs a Rust dylib through purego.
- **Git** ([report](research/git-jetbrains.md)): Use the git CLI for all writes. go-git has no stash or rebase, and its merge is fast-forward only. Build order:
  1. Status, gutter markers, diff.
  2. Commit window, partial staging, amend, rollback.
  3. Log, blame, file and selection history.
  4. Branches, sync, push dialog, stash, continue and abort.
  5. 3-way merge, interactive rebase.
  6. Worktrees, submodules, shelf.
- **GitHub PRs** ([report](research/github-prs.md)): Get the host and token from `gh auth`. Call the GitHub API from a Go HTTP client with ETag caching. Use `gh` commands only for simple actions (checkout, create, merge).

## Milestones

Done milestones are marked **Done**. Open items are marked **Open**, and partly done items are marked **Partly done**.

### Done

1. **Done** App shell: window per project, file tree, tabs, preview tabs, splits, session restore, recent projects, `editor` shell command.
2. **Done** Commands: registry, keymap, command palette, settings UI and JSON files.
3. **Done** Search: a Search Everywhere popup (Double Shift) with the tabs All, Actions, Files and Text, a preview pane, filters and Open in Find Window (the Find panel tab). Replace in Files is the Text tab with the replace field shown (Toggle Replace), a file mask and a scope; each match row has Replace and Replace All in File. The dialog is centered and resizable, and the activity bar has a Search icon that opens the Text tab. Recent Files (Cmd+E) lists recent files and tool windows. There is no Search sidebar view.
4. **Done** Git phases 1 and 2.
5. **Done** Git phases 3 and 4. Diffs use histogram with the indent heuristic, ignore-whitespace modes and moved-code detection.
6. **Done** Git phases 5 and 6, local history.
7. **Done** GitHub pull requests.
8. **Done** Large-file mode.
9. **Done** Problems/TODO, extra tokenizers, polish. Viewers for common non-text files: images (with zoom), SVG and Markdown preview, video, audio, PDF, and a hex view for other binary files.
    - **Done** Chosen from the [IDE research](research/ide-features.md): EditorConfig and folder settings, hot exit, bracket pair colors and guides, shortcuts that work on non-Latin keyboard layouts, untitled files (Cmd+N), and back and forward navigation (Ctrl+- and Ctrl+Shift+-). Ideas for later are in [IDEAS.md](../IDEAS.md).
    - **Done** UI redesign: a design system (tokens and components in `internal/design`, documented in `docs/design.md`), then the workbench and all views rebuilt on it: VS Code title bar with command center, activity bar at the top of the sidebar, VS Code Source Control (commit box at the top, tree view, Graph section), and VS Code tab labels for files with the same name.

### Open

10. **Open** Before the polish loop:
    - **Done** OpenTelemetry support: the app records traces and metrics (for example frame time, input latency, CPU, memory, search, Git and file load times) and writes them to local files, so that we can collect and compare performance data. Use the OTel Go SDK with the `stdouttrace`, `stdoutmetric` and `stdoutlog` exporters: one JSON object per line, one file per signal, written through the rotating `internal/applog.Writer`. Agents query the files with the DuckDB CLI (`read_json`); keep the SQL views in the repo. The stdout JSON has no stability guarantee, so pin the SDK version. Keep idle work at zero: use `ManualReader` for metrics and `SimpleSpanProcessor` for traces, because the batch processor and the periodic reader wake on timers. Do not hammer the disk: spans only for coarse work (search, Git, file load), never one for each frame or keystroke; frame time and input latency go into histograms; each file has a size cap with rotation; telemetry stays within the E2E disk-write budgets (`e2e/qa_budgets_test.go`).
    - **Done** Budgets in the real build: one table in the app code with a budget for each kind of action (search, Git, file load, input-to-frame latency, CPU, memory, disk writes). Check them on events (when a span ends or metrics are collected), never on a timer. When a value goes over, write a `budget.exceeded` record (action, value, budget) to the OTel logs file and one line to `editor.log`, once per budget per session with a count. Only log, never fail. The values differ from the E2E budgets, which belong to stress fixtures. The app has no cgo, so it cannot use `proc_pid_rusage` as the E2E harness does: use `syscall.Getrusage` for CPU time and peak RSS, `runtime/metrics` for the heap, and a byte count in our own writers for disk writes.
    - **Open** (blocked) Replace the MyGo soft wrap with VS Code word wrap that works the same. The current wrap (`editor.wordWrap`, Alt+Z) uses MyGo's soft wrap and does not yet match VS Code. First check how VS Code does it, in its source code and in the running app, and do not assume. Then do the same for all values of `editor.wordWrap`. Research done ([word-wrap.md](research/word-wrap.md)), and Alt+Z now works as in VS Code (a second toggle goes back to the setting). Blocked: the wrap points, the wrapped indent and the break classes are private in MyGo 0.10.0, so the VS Code algorithm needs a MyGo fork (a `replace` directive) or a wrap hook upstream.
    - **Done** Annotated review for agents, modeled on [Plannotator](https://plannotator.ai/). Annotate local changes (PR-style diff with a file tree and staging), Markdown plans (rendered preview, with versions and diffs between them), GitHub PR diffs and any open file. Annotation types as in Plannotator: comments, replacements or code suggestions, deletions, and a global comment. The finished review goes straight to a running agent, for example Claude Code through a hook or a watched file, as structured feedback. When we build it, read how Plannotator sends feedback to each agent in its code ([backnotprop/plannotator](https://github.com/backnotprop/plannotator), MIT or Apache 2.0). Done, as in Plannotator: only the agent starts a review, and the decision comes back on stdout. `editor review` (local changes with a diff type picker, Viewed marks and staging), `editor review <PR URL>`, `editor annotate` (Markdown files, annotated on their source lines) and `editor plan-hook` (a Claude Code PermissionRequest hook on ExitPlanMode) open a Review tab with inline comments and code suggestions, file and global comments, and Approve, Send Feedback and Approve with Notes. The feedback text is ported from Plannotator (`internal/review`, `internal/reviewui`). A skill (`skills/editor-review`) tells Claude Code when to ask for a review. By decision of the owner, a rendered Markdown preview and plan versions are out of scope, and an MCP server is an idea in IDEAS.md.
11. **Done** UI and UX polish loop: a subagent uses the editor as a VS Code or JetBrains user would, lists broken or rough UI and UX, and the findings are fixed. At most 3 rounds.
    - **Done** Two rounds. The first fixed the rough UI that a VS Code user finds. The second covered interaction gaps (keys in diffs, the Explorer, the palette), saved view state (pane sizes, splits, diff layout, find history), errors and microcopy, accessibility (screen reader labels, High Contrast themes), text input and macOS (Unicode marks, NFC matching, Option characters, the Help, Edit and Selection menus, Welcome). The UI stays English only. Open, blocked by MyGo 0.10.0: dragging tabs and files out of the window, document types next to the extension list, more than one action in a toast.
    - **Done** Skeletons that match the loaded view. Today each skeleton is built by hand, so row counts, columns, line shapes and sizes are guesses (for example one generic code skeleton for all code and diff views, and no commit box in the Source Control skeleton). Instead, a view renders its normal layout with a loading flag and mock data of the same shape (rows, columns, heights, counts where known), and the skeleton style turns the text into bars. A new view then gets a correct skeleton with no extra work. Code, diff, multi-file diff, patch, merge and Source Control skeletons now draw the real views with mock data (`feedback.CodeSkeleton`, `feedback.DiffSkeleton`); the list skeletons already used the views' own row builders.
    - **Done** The Git graph does not stay at the top when new commits are added. Check what VS Code does in its Source Control graph (keep the scroll position at the top or keep the selected commit in view), then do the same. As in VS Code, a graph at the top shows the new commits; a scrolled graph keeps its place and its Refresh button shows "Outdated".
12. **Partly done** E2E loop: a reporter subagent runs and explores the E2E suite, an implementor subagent fixes bugs and adds tests. At most 3 rounds.
    - **Done** Stress tests for each feature (file tree, tabs and splits, palette, Search Everywhere, Git, pull requests, large files, settings). They run in the E2E harness with large generated inputs, for example 100k-file projects, deep Git history, 1 GB files and many tabs. Large file tree, 200 tabs, the Git graph, and a 1 GB file, a 50 MB single-line file and a 200k-line file are in `e2e/qa_stress_test.go`. Pull requests (1000 PRs, a PR with 300 files), the issue list (1000 issues) and the notification list (500) are in `e2e/qa_stress_github_test.go` and `e2e/qa_stress_issues_test.go`. The other files hold 8 editor groups with 50 tabs each (`qa_stress_splits_test.go`), the Command Palette and Search Everywhere over the 100k-file tree (`qa_stress_search_test.go`), the Settings and Keyboard Shortcuts lists with 2160 user bindings (`qa_stress_settings_test.go`), 20k TODO comments and 10k problems (`qa_stress_insights_test.go`), a file with 500 history versions (`qa_stress_history_test.go`), 200 unsaved buffers to restore (`qa_stress_hotexit_test.go`), word wrap (`qa_stress_wrap_test.go`) and a branch switch that rewrites 10k files (`qa_stress_git_test.go`). Every scenario checks that its input reached the app (a dialog, a result row, a tab) before it records numbers.
    - **Done** A GitHub mock for the E2E harness: a local fake of the GitHub API that the app uses in place of github.com, with fixtures for issues (many labels, long titles, milestones), notifications and pull requests (reviews, checks, comments). The GitHub views then get the same E2E, overflow and redline coverage as the other views. The mock is in `e2e/githubmock*_test.go`; the app uses it through `EDITOR_E2E_GITHUB_URL` and `EDITOR_E2E_GITHUB_TOKEN` (E2E builds only).
    - **Done** Each stress test records the app's CPU time and peak resident memory (from the app process), idle CPU after the work, responsiveness (time from input to the settled frame), disk bytes read and written by the app process (macOS proc_pid_rusage), and the disk space that the app's own data takes (user data, local history, shelf, caches, temp files). The results go to a JSON report next to the E2E artifacts (`resources.json`), compared against budgets in the test (`e2e/qa_budgets_test.go`), so that regressions show and we can act on them. A budget is the highest value of at least three runs of the full perf run (`EDITOR_E2E_PERF=1`, 8 scenarios in parallel), times 2 for time and CPU (rounded up to 5 ms, startup CPU to 50 ms), times 1.25 for peak memory (rounded up to a MiB), times 2 for disk bytes and data size (rounded up to 64 KiB, or a MiB from 1 MiB), and times 2 for the idle CPU share. The floors are 50 ms latency, 30 ms first frame, 10 ms CPU, 64 KiB disk and data, and 2.5% idle CPU.
    - **Partly done** Lower the base CPU and memory use. The app uses about 120 MB when idle and 100 to 300 MB in normal use. Profile idle and typical sessions (heap and CPU), remove the largest costs, and lower the budgets to the new numbers. Done: startup runs `git status` and `git ls-files` once instead of twice (13 git calls instead of 17 on the demo project), a write to `session.json` no longer reloads the settings, and five startup CPU budgets are lower. Open: about 125 MB of the idle memory is GPU memory. The MyGo caret blink (530 ms, private in MyGo 0.10.0) asks for frames all the time, so the Metal renderer never frees its textures. With the window hidden, the app uses 75 MB. A fix needs a blink timeout or texture release in MyGo (a fork or upstream).
    - **Done** Search causes CPU spikes: while a search runs, and when you scroll the search results, the results editor view and similar views. Profile these paths and remove the spikes. Text search used one worker for each core but one; more than 4 workers only fought over file reads. It now uses at most 4, which cuts the search CPU by 3.6 times at the same speed. Scrolling the results and the preview takes about 1 ms for each frame, almost all of it MyGo paint and layout, so it needed no change.

Every feature that writes to disk has a size cap and pruning (for example local history, shelf, caches), writes only on change, and never fills the disk: it checks free space before large writes, such as saving a 1 GB file through a temp file.

Every milestone keeps UI polish in mind: native macOS spacing, states (hover, focus, empty, loading, error) and keyboard access.
