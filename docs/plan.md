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

1. App shell: window per project, file tree, tabs, preview tabs, splits, session restore, recent projects, `editor` shell command.
2. Commands: registry, keymap, command palette, settings UI and JSON files.
3. Search: one Search view in the sidebar with scopes All, Files, Symbols, Text, Actions and Git. Text search has multi-line queries, replace with preview, and include and exclude globs. The Command Palette stays a popup.
4. Git phases 1 and 2.
5. Git phases 3 and 4. Diffs use histogram with the indent heuristic, ignore-whitespace modes and moved-code detection.
6. Git phases 5 and 6, local history.
7. GitHub pull requests.
8. Large-file mode.
9. Problems/TODO, extra tokenizers, polish. Viewers for common non-text files: images (with zoom), SVG and Markdown preview, video, audio, PDF, and a hex view for other binary files.
    - Chosen from the [IDE research](research/ide-features.md): EditorConfig and folder settings, hot exit, bracket pair colors and guides, scratch files, shortcuts that work on non-Latin keyboard layouts, untitled files (Cmd+N), and back and forward navigation (Ctrl+- and Ctrl+Shift+-). Ideas for later are in [IDEAS.md](../IDEAS.md).
    - UI redesign: a design system (tokens and components in `internal/design`, documented in `docs/design.md`), then the workbench and all views rebuilt on it: VS Code title bar with command center, activity bar at the top of the sidebar, VS Code Source Control (commit box at the top, tree view, Graph section), and VS Code tab labels for files with the same name.
10. UI and UX polish loop: a subagent uses the editor as a VS Code or JetBrains user would, lists broken or rough UI and UX, and the findings are fixed. At most 3 rounds.
11. E2E loop: a reporter subagent runs and explores the E2E suite, an implementor subagent fixes bugs and adds tests. At most 3 rounds.
    - Stress tests for each feature (file tree, tabs and splits, palette, Search Everywhere, Git, pull requests, large files, settings). They run in the E2E harness with large generated inputs, for example 100k-file projects, deep Git history, 1 GB files and many tabs.
    - Each stress test records the app's CPU time and peak resident memory (from the app process), idle CPU after the work, responsiveness (time from input to the settled frame), disk bytes read and written by the app process (macOS proc_pid_rusage), and the disk space that the app's own data takes (user data, local history, shelf, caches, temp files). The results go to a JSON report next to the E2E artifacts, compared against budgets in the test, so that regressions show and we can act on them.

Later: an optional structural diff mode that compares syntax trees (like difftastic), so formatting-only changes do not show as changes.

Every feature that writes to disk has a size cap and pruning (for example local history, shelf, caches), writes only on change, and never fills the disk: it checks free space before large writes, such as saving a 1 GB file through a temp file.

Every milestone keeps UI polish in mind: native macOS spacing, states (hover, focus, empty, loading, error) and keyboard access.
