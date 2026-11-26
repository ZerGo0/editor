# Search Everywhere: custom Go search or "fff"?

Date: 2026-11-26. Scope: research only. Language: ASD-STE100 Simplified Technical English.

## 0. Short answer

Build the search in Go. Use "fff" only as an optional, later replacement for the Files and Text engines behind one Go interface.

Reasons:

1. fff does only two of your four tabs (Files, Text). It has no Symbols tab and no Actions tab.
2. fff is a Rust library with a C ABI. You can load it with purego, but you then ship a 6 to 10 MB dylib, a second allocator, a second file watcher, an LMDB database and a copy of libgit2. Your goal is low memory and CPU, and your own Git integration needs its own Git layer.
3. fff reads at most 10 MB per file by default. Your project has the 1 GB file goal. The editor must handle those files itself.
4. The Go parts you need are small. A fzf-quality matcher is one MIT package. MyGo already has a file watcher, a virtualized list and a popup. The match step is not the hard part. The index, the ranking and the UI are the hard parts, and fff does not solve the UI.
5. fff is pre-1.0 (0.11.0). Its C API already has deprecated functions.

## 1. What is "fff"

### 1.1 Candidates

| Repository | What it is | Fit |
|---|---|---|
| [dmtrKovalenko/fff](https://github.com/dmtrKovalenko/fff) (11.3k stars, MIT, Rust, pushed 2026-11-25) | File search toolkit. It started as fff.nvim. The README says "Package name changed from `fff.nvim` to `fff`". | Yes. This is the one. |
| [dylanaraps/fff](https://github.com/dylanaraps/fff) | File manager in bash | No |
| [tomnomnom/fff](https://github.com/tomnomnom/fff) | Fast HTTP fetcher in Go | No |
| [meekrosoft/fff](https://github.com/meekrosoft/fff) | Fake Function Framework for C tests | No |

Sources: GitHub search and repository metadata (`gh api repos/dmtrKovalenko/fff`). The repository is MIT: [LICENSE](https://github.com/dmtrKovalenko/fff/blob/main/LICENSE).

### 1.2 Architecture

The repository README calls it "a file search toolkit for humans and AI agents" ([README](https://github.com/dmtrKovalenko/fff#readme)). The Rust workspace has these crates ([Cargo.toml](https://github.com/dmtrKovalenko/fff/blob/main/Cargo.toml)):

- `fff-core` (published as `fff-search`): the engine. Main type is `FilePicker`. It scans in a background thread, keeps a sorted file list, watches the file system and searches ([lib.rs](https://github.com/dmtrKovalenko/fff/blob/main/crates/fff-core/src/lib.rs)).
- `fff-grep`: content search (plain SIMD memmem, regex, fuzzy Smith-Waterman, multi-pattern Aho-Corasick).
- `fff-query-parser`: query language with constraints such as `*.rs !test/ git:modified`.
- `fff-c`: C ABI, built as `cdylib` and `staticlib` ([Cargo.toml](https://github.com/dmtrKovalenko/fff/blob/main/crates/fff-c/Cargo.toml)).
- `fff-mcp`, `fff-nvim`, `fff-python`, plus Node and Bun packages.

Main building blocks in the core:

- Path fuzzy match: `neo_frizbee` (a SIMD Smith-Waterman matcher, derived from [saghen/frizbee](https://github.com/saghen/frizbee), MIT).
- Files list: one contiguous arena of path chunks (README, "Performance choices").
- Walk: `ignore` crate (ripgrep rules), optional `zlob` (needs Zig to build).
- Git: `git2` with vendored libgit2.
- Watch: `notify` crate with a debouncer.
- Persistence: LMDB (`heed`) for frecency and query history ([dbs/](https://github.com/dmtrKovalenko/fff/tree/main/crates/fff-core/src/dbs)).
- Threads: a rayon search pool (performance cores only on macOS) and a rayon background pool (half of the cores) ([parallelism.rs](https://github.com/dmtrKovalenko/fff/blob/main/crates/fff-core/src/parallelism.rs)).
- Allocator: mimalloc.

### 1.3 Matching and ranking

Source: [score.rs](https://github.com/dmtrKovalenko/fff/blob/main/crates/fff-core/src/score.rs) and [dbs/frecency.rs](https://github.com/dmtrKovalenko/fff/blob/main/crates/fff-core/src/dbs/frecency.rs).

- Typo resistance: the README says the matcher "survives dropped characters and reorderings". The query has a `max_typos` setting (tests use 0, 2 and 4).
- Multi-part queries: each space-separated part matches separately.
- Score parts for each file: base fuzzy score, frecency boost, git-status boost (modified files get a boost), distance penalty from the current file, filename bonus (exact file name), special-filename bonus, current-file penalty, and a "combo" boost for files that the user opened often for the same query.
- Frecency: sum of `exp(-0.0693 * days_ago)` over accesses, so the half-life is 10 days. It uses only the last 30 days. In AI mode the half-life is 3 days and the window is 7 days.
- Git worktrees share frecency with the main checkout.
- Content search: smart case, auto fuzzy fallback when plain search finds nothing, "definition first" classifier for Rust, Python, Go, JS/TS, C/C++, Java, and others ([README, definition search](https://github.com/dmtrKovalenko/fff#readme)).

### 1.4 Limits that matter for you

[constants.rs](https://github.com/dmtrKovalenko/fff/blob/main/crates/fff-core/src/constants.rs):

- Largest file for grep and for the mmap content cache: 10 MB.
- Largest file for the bigram content index: 2 MB.
- Content cache: up to 512 MB of mapped files for a small repository.
- Watcher rescans at most every 30 s (5 min at 1,000,000 files).

Memory, from the README ("Memory allocation"): content index about 360 bytes per file (about 36 MB for 100k files); "about 26 MB resident" on a 14k-file repository; "a few hundred MB" on a 500k-file repository. These are the author's numbers. I did not measure them.

### 1.5 C ABI and library form

Yes. [include/fff.h](https://github.com/dmtrKovalenko/fff/blob/main/crates/fff-c/include/fff.h) exports plain C functions: `fff_create_instance_with(const FffCreateOptions*)`, `fff_search`, `fff_search_directories`, `fff_search_mixed`, `fff_live_grep`, `fff_multi_grep`, `fff_wait_for_scan`, `fff_get_scan_progress`, `fff_track_query`, `fff_refresh_git_status`, `fff_watch` (callback), `fff_destroy` and the matching `fff_free_*` functions. Every call returns a heap `FffResult` that you must free. Results are C structs (arrays of items with strings owned by the parent).

Options (`FffCreateOptions`) include: `base_path`, `frecency_db_path`, `history_db_path`, `enable_content_indexing`, `watch`, `enable_mmap_cache`, cache budgets, `follow_symlinks`. Options `extra_ignore`, `disable_git` and `disable_ignore_files` need the `zlob` build.

Prebuilt libraries exist in the release assets (for example release 0.11.1-nightly.eb58401): `c-lib-aarch64-apple-darwin.dylib` is 5.8 MB; the x86_64 macOS one is 10.1 MB ([releases](https://github.com/dmtrKovalenko/fff/releases)). The CI script signs them ad hoc and sets the install name to `@rpath/libfff_c.dylib` ([release.yaml](https://github.com/dmtrKovalenko/fff/blob/main/.github/workflows/release.yaml)). The latest stable tag is 0.11.0 (2026-09-21). Most recent builds are nightly pre-releases.

No Go binding exists in the repository. The README says "Go via cgo". I found no purego binding (GitHub search for "fff go bindings" returned nothing).

### 1.4b Can a pure-Go app use it?

Yes, technically.

- purego loads a dylib without cgo: [ebitengine/purego](https://github.com/ebitengine/purego) (Apache-2.0).
- MyGo already depends on purego v0.11.1 ([go.mod](https://github.com/egoist/mygo/blob/main/go.mod)). MyGo has a documented path to bind a C library or a Rust crate: package `ffi`, and a `mygo.json` entry `"build": { "cargo": "./rust" }`. The CLI builds the library, puts it in the app, and signs it from the inside out with the hardened runtime ([docs/native-modules.md](https://github.com/egoist/mygo/blob/main/docs/native-modules.md), [docs/distribution.md](https://github.com/egoist/mygo/blob/main/docs/distribution.md)). There is an example: [examples/rust-native](https://github.com/egoist/mygo/tree/main/examples/rust-native).
- See does not use purego for search. It uses purego only for AppKit symbols ([see/internal/app/symbols_darwin.go](https://github.com/wibus-wee/see/blob/main/internal/app/symbols_darwin.go)).

Costs and risks of this path:

| Topic | Effect |
|---|---|
| Build | Use a prebuilt dylib (easy, but nightly and unpinned) or add a thin Rust crate that depends on `fff-search` and build it with MyGo's cargo path (needs rustup; fff recommends Zig for the best build). |
| Size | 6 MB (arm64) to 10 MB (x86_64) per architecture. |
| Signing and notarization | Re-sign with your Developer ID. MyGo does this for libraries it builds. |
| Binding code | You must read C structs (`FffSearchResult`, `FffGrepResult`) by unsafe pointer offsets in Go. MyGo docs say purego handles structs of simple types and pointers; nested arrays need manual reads. Callbacks (`fff_watch`) need purego callbacks. I did not test this. |
| Memory | A second allocator (mimalloc), a rayon thread pool, an LMDB environment, a copy of libgit2, and a full file list plus content index. Roughly 26 MB for 14k files; a few hundred MB for 500k files (author's numbers). |
| Duplicate work | fff runs its own watcher and its own Git status. MyGo already has a file watcher and you need your own Git layer. |
| Failure mode | A Rust panic or abort in the dylib kills the whole editor. |
| API stability | Version 0.11.0; `fff_create_instance` is already deprecated; nightly builds are frequent. |
| Debug | Two runtimes and two stack types. |

Gain: very fast Files and Text search, typo resistance, frecency and Git status in one call. The author says path search is "around 10 times faster than fzf" and content search 20 to 50 times faster than ripgrep on macOS (fff-core [README](https://github.com/dmtrKovalenko/fff/blob/main/crates/fff-core/README.md)). That claim is for repeated searches in a long-running process. I did not verify it.

## 2. Go options

### 2.1 Fuzzy matchers

| Package | License | Notes |
|---|---|---|
| [junegunn/fzf](https://github.com/junegunn/fzf) `src/algo` | MIT | The fzf algorithm (FuzzyMatchV2 is a modified Smith-Waterman with bonuses for word starts and camelCase, and a gap penalty). The package is importable (it is not under `internal`), but the API is not stable and the module pulls many dependencies into `go.mod`. It has ASM fast paths for arm64 and amd64 ([algo.go](https://github.com/junegunn/fzf/blob/master/src/algo/algo.go)). Best choice to import or to port (MIT allows both). |
| [sahilm/fuzzy](https://github.com/sahilm/fuzzy) | MIT | Simple. Scores like Sublime Text. Returns match positions. Last push 2026-06. Slower (see benchmark). See uses it ([see/internal/search](https://github.com/wibus-wee/see/tree/main/internal/search), idea only). |
| [lithammer/fuzzysearch](https://github.com/lithammer/fuzzysearch) | MIT | Subsequence match with Levenshtein distance. No path-aware scoring. |
| [ktr0731/go-fuzzyfinder](https://github.com/ktr0731/go-fuzzyfinder) | MIT | Terminal UI. Not useful. |
| MyGo `ui/editor/lsp.go` `fuzzyScore` | MIT | Small Zed-style subsequence scorer for completion lists. Unexported. |
| Own port of frizbee-style typo resistance | n/a | Possible later. frizbee is MIT ([saghen/frizbee](https://github.com/saghen/frizbee)). |

My local benchmark (Apple M4, Go 1.27.4, 4,610 paths from the repositories cloned for this research, query `edtrvw`, scratch code in `<scratch>/bench-fuzzy-grep`):

| Matcher | Time for 4,610 paths |
|---|---|
| sahilm/fuzzy `Find` (with positions, allocating) | 2.13 ms (about 460 ns per path) |
| fzf `FuzzyMatchV2`, one thread, no positions | 0.32 ms (about 69 ns per path) |

Linear estimate for 1,000,000 paths: about 460 ms for sahilm/fuzzy; about 70 ms for fzf on one thread; less with 8 threads. This is a rough number from a small set. The parallel fzf run was slower on 4,610 paths because the work was too small to split. Re-measure on a real 500k-path list before you decide.

### 2.2 File walking and ignore rules

| Option | License | Notes |
|---|---|---|
| `git ls-files -z --cached --others --exclude-standard` | n/a | Exact Git rules (nested .gitignore, global excludes, `.git/info/exclude`), one process, fast. See uses a similar Git call ([see/internal/git/ignore.go](https://github.com/wibus-wee/see/blob/main/internal/git/ignore.go)). Needs the git binary. Use it inside repositories. |
| [charlievieth/fastwalk](https://github.com/charlievieth/fastwalk) | MIT | Parallel `WalkDir`. README claims about 2.5x faster on macOS. Use it outside repositories and for non-Git folders. |
| [go-git/go-git](https://github.com/go-git/go-git) `plumbing/format/gitignore` | Apache-2.0 | Pure-Go matcher with Git semantics. Large dependency if you import the whole module. |
| [boyter/gocodewalker](https://github.com/boyter/gocodewalker) | MIT | Walker with .gitignore and .ignore support. Small project (98 stars). |
| [sabhiram/go-gitignore](https://github.com/sabhiram/go-gitignore), [denormal/go-gitignore](https://github.com/denormal/go-gitignore) | MIT | Matchers. No push since 2024 and 2023. |
| `filepath.WalkDir` | BSD-3 | Standard library. Serial. |

### 2.3 Fast text search

| Option | License | Notes |
|---|---|---|
| Own parallel scan: worker pool, mmap or `os.ReadFile`, `bytes.Index` for literals, `regexp` for patterns | BSD-3 | My test: 4,610 files (79 MB incl. everything, files above 4 MB skipped), literal `WatchFiles`, 10 workers, files read from the OS page cache each run: 47 ms per full scan. `rg -l` on the same tree: 47 ms wall. So plain Go is in the same range as ripgrep for a literal on a small tree. |
| Go standard `regexp` | BSD-3 | Linear time, but slow. Use it only for real regex queries. Faster forks: [grafana/regexp](https://github.com/grafana/regexp) (BSD-3, actively pushed), [coregx/coregex](https://github.com/coregx/coregex) (MIT, SIMD, 320 stars, young). Test both before you trust them. |
| [google/codesearch](https://github.com/google/codesearch) | BSD-3 | Trigram index on disk (posting lists, mmap, delta updates) and a regexp-to-trigram query builder in `index/regexp.go`. The repository is archived (read-only), but the code is small and BSD. Good model for an optional index. Format described in [index/read.go](https://github.com/google/codesearch/blob/master/index/read.go). |
| [sourcegraph/zoekt](https://github.com/sourcegraph/zoekt) | Apache-2.0 | Trigram code search with shards and ranking. Heavy (server-oriented). Read it for ranking ideas. |
| [blevesearch/bleve](https://github.com/blevesearch/bleve) | Apache-2.0 | Full-text engine. Too heavy for this use. |
| Bundle the `rg` binary and spawn it | Unlicense/MIT ([ripgrep](https://github.com/BurntSushi/ripgrep)) | Lowest effort, best raw speed, zero resident memory. See does this but needs the user to install ripgrep ([see/internal/search/search.go](https://github.com/wibus-wee/see/blob/main/internal/search/search.go), idea only). If you bundle it, there is no install step. Cost: one process per search, about 5 MB binary, JSON output parsing. |

### 2.4 Index updates

- fsnotify ([fsnotify/fsnotify](https://github.com/fsnotify/fsnotify), BSD-3) uses kqueue on macOS. Its README says kqueue "requires opening a file descriptor for every file that's being watched". It is a poor fit for large trees. It lists FSEvents as "needs support in x/sys/unix".
- You do not need fsnotify. MyGo provides `mygo.WatchFiles` (see section 4). On macOS it uses FSEvents.

## 3. How JetBrains Search Everywhere works

Primary sources: [help page](https://www.jetbrains.com/help/idea/searching-everywhere.html) and the [intellij-community source](https://github.com/JetBrains/intellij-community/tree/master/platform/lang-impl/src/com/intellij/ide/actions/searcheverywhere).

Facts from the help page:

- Open with Shift twice. By default the list shows recent files.
- Tab switches the scope: All, Classes, Files, Symbols, Actions, and Text. Extra tabs (for example Git) are possible through Advanced Settings.
- Direct shortcuts: Ctrl+N (class), Ctrl+Shift+N (file), Ctrl+Alt+Shift+N (symbol), Ctrl+Shift+A (action).
- Shift twice again (or Alt+N) turns on "Include non-project items". In other tabs the option is "All Places".
- Text search is a tab. On the All tab, text matches show at the bottom "when there are few or no other search results".
- Ctrl+Down jumps to the "more..." rows.
- A filter button narrows results (for example by file type). A button sends results to the Find tool window.
- Query prefix `/` shows settings groups. Simple math in the query is evaluated.

Facts from the source:

- Each tab is built from `SearchEverywhereContributor` objects. A contributor has an ID, a "sort weight" for its group, and it produces weighted elements ([SearchEverywhereContributor.java](https://github.com/JetBrains/intellij-community/blob/master/platform/lang-api/src/com/intellij/ide/actions/searcheverywhere/SearchEverywhereContributor.java)). The Javadoc says the contributor weight orders groups, "not elements".
- Group sort weights in the source: Classes 100, Files 200, Symbols 300, Actions 400, Text 1500 ([ClassSearchEverywhereContributor.kt](https://github.com/JetBrains/intellij-community/blob/master/platform/lang-impl/src/com/intellij/ide/actions/searcheverywhere/ClassSearchEverywhereContributor.kt), [FileSearchEverywhereContributor.kt](https://github.com/JetBrains/intellij-community/blob/master/platform/lang-impl/src/com/intellij/ide/actions/searcheverywhere/FileSearchEverywhereContributor.kt), [ActionSearchEverywhereContributor.kt](https://github.com/JetBrains/intellij-community/blob/master/platform/lang-impl/src/com/intellij/ide/actions/searcheverywhere/ActionSearchEverywhereContributor.kt), [TextSearchContributor.kt](https://github.com/JetBrains/intellij-community/blob/master/platform/lang-impl/src/com/intellij/find/impl/TextSearchContributor.kt)). The Symbols value (300) is in `SymbolSearchEverywhereContributor.java`.
- Element order on the All tab: sort by element priority (match quality), and break ties with the contributor sort weight ([SearchEverywhereFoundElementInfo.java](https://github.com/JetBrains/intellij-community/blob/master/platform/lang-api/src/com/intellij/ide/actions/searcheverywhere/SearchEverywhereFoundElementInfo.java)). Some contributors add a small bonus to the priority (Files +2, Classes +5, Recent files +5).
- Limits per contributor: 15 results when several contributors run (All tab), 30 when one runs (single tab). "More..." loads another page of that size ([SearchEverywhereUI.java](https://github.com/JetBrains/intellij-community/blob/master/platform/lang-impl/src/com/intellij/ide/actions/searcheverywhere/SearchEverywhereUI.java)).
- Search is concurrent: `MixedResultsSearcher` runs all contributors in parallel, collects results in a `ResultsAccumulator` with a per-contributor limit and a "has more" flag, and pushes them to the list as they arrive ([MixedResultsSearcher.java](https://github.com/JetBrains/intellij-community/blob/master/platform/lang-impl/src/com/intellij/ide/actions/searcheverywhere/MixedResultsSearcher.java)). A new keystroke cancels the old search.
- Duplicate results from different contributors are merged by equality providers (for example a file found as a file and as a recent file) ([SEResultsEqualityProvider.kt](https://github.com/JetBrains/intellij-community/blob/master/platform/lang-impl/src/com/intellij/ide/actions/searcheverywhere/SEResultsEqualityProvider.kt)).
- Recent files: `RecentFilesSEContributor` reads the editor history (newest first), filters by the pattern, and sorts by weight ([RecentFilesSEContributor.java](https://github.com/JetBrains/intellij-community/blob/master/platform/lang-impl/src/com/intellij/ide/actions/searcheverywhere/RecentFilesSEContributor.java)).
- Name matching uses `MinusculeMatcher`: camel-hump and snake_case matching, space and `*` as wildcards, and a fallback to simple matching for long prefixes to avoid pauses ([MinusculeMatcherImpl.kt](https://github.com/JetBrains/intellij-community/blob/master/platform/util/text-matching/src/com/intellij/psi/codeStyle/MinusculeMatcherImpl.kt)).
- Search history per tab is kept (`SearchHistoryList`). An optional ML ranking layer exists (`SearchEverywhereMlService`). The source lists spell-check "correction" wrappers for typo help.
- Not verified: the exact algorithm for double Shift detection and the default time gap. The help page says only "Press Shift twice".

## 4. What MyGo already gives you

Checked in the repository [egoist/mygo](https://github.com/egoist/mygo) (MIT, clone read at the date above).

| Need | MyGo item | Source |
|---|---|---|
| Fuzzy matcher | Only an unexported Zed-style `fuzzyScore` in the editor completion code. `ui.Combobox` and `ui.Autocomplete` filter with prefix first, then contains. No reusable fuzzy package. | `ui/editor/lsp.go`, [combobox.md](https://github.com/egoist/mygo/blob/main/docs/ui/combobox.md), [autocomplete.md](https://github.com/egoist/mygo/blob/main/docs/ui/autocomplete.md) |
| Virtualized list | `ui.List`: builds only visible rows, "a list of millions of rows is as fast as one of ten", variable row height, `ListState` with `Key` (selection follows the item), keyboard choose. Perf table: 14.5 microseconds for the visible rows of a million-row list. | [list.md](https://github.com/egoist/mygo/blob/main/docs/ui/list.md), [performance.md](https://github.com/egoist/mygo/blob/main/docs/ui/performance.md) |
| Table, tree, grid | `ui.Table`, `ui.Tree`, `ui.GridView` | `ui/table.go`, `ui/tree.go`, `ui/gridview.go` |
| Popup field with item selection | `ui.ComboboxBase` (an input whose text filters the items of a popup, arrows highlight, Enter chooses) | [combobox.md](https://github.com/egoist/mygo/blob/main/docs/ui/combobox.md) |
| Command palette window | "Panels" (launcher or command palette windows, floating, global shortcut) | [docs/windows.md](https://github.com/egoist/mygo/blob/main/docs/windows.md) |
| Global shortcut | `mygo.GlobalShortcut.Register("CmdOrCtrl+Shift+Space", fn)` | [docs/native.md](https://github.com/egoist/mygo/blob/main/docs/native.md) |
| Double Shift | `HandleInput` and `ObserveInput` get modifier keys pressed and released alone (`ui.CodeShiftLeft`, `InputKeyDown`, `InputKeyUp`). You count two taps in a time window. | [docs/ui/input.md](https://github.com/egoist/mygo/blob/main/docs/ui/input.md) |
| File watching | `mygo.WatchFiles`: `Recursive`, `Ignore(path,isDir)` callback, `Debounce` 50 ms, `MaxDelay` 500 ms, `MaxEntries` 65,536 on macOS (snapshot size), `HintsOnly` mode (no snapshot, only directory hints). macOS backend is FSEvents. Use `Next(ctx)` on its own goroutine. | [docs/native.md](https://github.com/egoist/mygo/blob/main/docs/native.md), `watch.go`, `internal/darwin/watch.go` |
| Main thread rule | `Window.Update(fn)` runs a function on the main thread from other goroutines. | [docs/ui/views.md](https://github.com/egoist/mygo/blob/main/docs/ui/views.md) |
| Find bar | `ui.FindBar` (in-file find UI). The app counts matches. | [find-bar.md](https://github.com/egoist/mygo/blob/main/docs/ui/find-bar.md) |
| Highlighter | Line-based `Tokenizer`, or your own `Highlighter`. Good source for a symbol scanner. | [code-editor.md](https://github.com/egoist/mygo/blob/main/docs/ui/code-editor.md) |
| Native library path | `ffi` package and cargo/zig/swift build entries | [native-modules.md](https://github.com/egoist/mygo/blob/main/docs/native-modules.md) |

MyGo has no file index, no ranking, no gitignore parser, and no command registry for a palette. You write these.

## 5. How See does it (ideas only; See has no license, so copy nothing)

Source: [wibus-wee/see](https://github.com/wibus-wee/see), `internal/search` and `internal/app/find.go`.

- Files: a list of relative paths comes from `rg --files` (or a directory walk if ripgrep is missing), capped by a maximum count, with ignored folders skipped. Ranking uses sahilm/fuzzy. Terms split on spaces. A term with `/` matches in path order. Other terms prefer the file name ("name first" view of the path). Matching is sharded over up to 8 goroutines and the shard results merge.
- One "Open Quickly" panel has modes by query prefix: `>` commands (matched on id, English name and localized title, best of three), `#` workspace symbols, `@` symbols in the current file, a trailing `:line:col` jump, and recent tabs. This is the same idea as the JetBrains All tab, with prefixes in place of tabs.
- Symbols: from LSP (`WorkspaceSymbols`, `documentSymbol`). You will not use LSP, so you need your own scanner.
- Text: spawns `rg --json` with a 4 MB file size limit and a 4000 column limit, streams per-file results, cancels with a context. The user must install ripgrep.
- Ignore data: `git ls-files --others --ignored` plus `git check-ignore`.
- Watching: `mygo.WatchFiles` (through a small wrapper).
- Stale results: a generation counter `gen` and `a.post.Post(func)` to return to the UI thread.

## 6. Recommendation

1. Write your own Search Everywhere in Go. Keep Files, Actions, Symbols and Text behind one interface (`Source`) so a source can be replaced later.
2. Matcher: import or port fzf `src/algo` (MIT). Use it for Files, Actions and Symbols. Add bonuses from fff and JetBrains (frecency, Git-modified, open tab, near the current file).
3. Text tab: start with an own parallel scan (bytes.Index, mmap, skip binary and huge files). Add a bundled `rg` only if the Go scan is too slow on your target trees. Add a trigram index only for trees above a size you measure (idea: codesearch format, in memory, built lazily).
4. Run a one-day spike for fff before you finalize: load the prebuilt `libfff_c.dylib` with purego, call `fff_search` and `fff_live_grep` on a 500k-file tree, and record resident memory, first-result latency, and crash behavior. Keep fff only for the Files and Text engines if it beats the Go version by a large factor and you accept the cost list in section 1.4b.
5. Do not use fsnotify. Use `mygo.WatchFiles`.

## 7. Rough design

### 7.1 Packages

```
search/
  match/      fzf-style matcher, query parser (terms, prefixes, constraints)
  index/      file index (arena), ignore rules, walker, watcher feed
  frecency/   access log, decay score, small on-disk file
  files/      Files source
  actions/    command registry and Actions source
  symbols/    per-language definition scanners, symbol store, Symbols source
  text/       Text source (scan, optional trigram)
  everywhere/ contributors, merge, limits, "more", history
  ui/         double-Shift detector, panel, List rows
```

### 7.2 Common types

- `Source` interface: `ID()`, `SortWeight() int`, `Search(ctx, query, limit, emit func(Result))`.
- `Result`: `Kind`, `Title`, `Detail`, `MatchPositions []uint16`, `Priority int32`, `Open func()`.
- All tab: start every source in parallel with one `context` per keystroke. Per-source limit 15 on the All tab and 30 on a single tab (JetBrains values). Sort by priority, break ties by source weight. Add a "more..." row. Merge duplicates by path.
- Empty query: show recent files (newest first), then recent actions.
- Stale results: a generation counter and `Window.Update` to publish.

### 7.3 Files tab

Data:

- `pathArena []byte`, `offs []uint32` (start of each path), `nameOff []uint16` (start of the file name), `mtime []int64`, `size []uint64`, `flags []uint8` (git status, open, ignored), `frecency []float32`.
- One snapshot pointer swapped atomically (copy-on-write for the file list). Updates from the watcher go into a small overlay slice, and a rebuild merges them.

Build:

- In a Git repository: `git ls-files -z --cached --others --exclude-standard`. Else: `fastwalk` with an ignore list (`.git`, `node_modules`, build output).
- Git status: `git status --porcelain=v2 -z`, run in the background, again on watcher hints.

Watch:

- `mygo.WatchFiles` with `Ignore` for ignored directories. For very large trees use `HintsOnly` and re-list only the hinted directories. The default snapshot limit (65,536 entries on macOS) is lower than a big repository.

Search:

- Split the query on spaces. A term with `/` matches the full path. Other terms match the file name first, then the path.
- Shard the arena across `min(GOMAXPROCS-1, 8)` goroutines. Each shard keeps a top-K heap. Merge the heaps.
- Score = fzf score + file-name bonus + frecency (10 day half-life, like fff) + Git-modified boost + open-tab boost - distance from the current file.
- Pre-filter by a character bitmask per path (26 letters plus digit and punctuation) to skip most paths with one AND operation.
- Constraints after the fff idea: `git:modified`, `*.go`, `!test/`.

Threads: 1 indexer goroutine, 1 watcher goroutine, N short query goroutines (cancelled by `ctx`), UI on the main thread.

### 7.4 Actions tab

- A registry: `Action{ID, Title, Category, Shortcut, Enabled func(ctx) bool, Run func()}`. The same registry feeds the menu, the keymap and the command palette (one source of truth).
- Match against title words and the dotted ID (See does this with three spellings). Rank = match score + usage count with decay + "enabled" bonus. Show the shortcut at the row end.
- In memory only. The list is small (hundreds). A single goroutine is enough.
- Settings entries can share the registry with a `/` prefix (JetBrains idea).

### 7.5 Symbols tab (no LSP, no tree-sitter)

- Reuse the line-based lexer from `ui/editor/lexers.go`. Add a per-language "definition rule" table: keyword then name (`func`, `type`, `class`, `def`, `fn`, `struct`, `interface`, markdown headings, and so on). fff has the same idea: a byte-level definition classifier with per-language keyword rules ([definition/languages.rs](https://github.com/dmtrKovalenko/fff/blob/main/crates/fff-core/src/definition/languages.rs)). You can read it for the rule list.
- Store: `names []string` interned in an arena, `kind []uint8`, `fileID []uint32`, `line []uint32`, `container []uint32`. Matcher runs on names only (small input: fast).
- File symbols (`@` prefix or current file): scan the open buffer on its version number, and cache by version.
- Workspace symbols: background goroutine, low priority, bounded by file size (for example skip files above 2 MB), scans files in the Files index in order of frecency, and re-scans a file when the watcher or an editor save reports a change. Persist nothing at first.
- Large files (1 GB class): never scan them for workspace symbols. For the open buffer scan only visible chunks.
- Tie-break by kind (types before fields), exact-case match, and shorter name.

### 7.6 Text tab

Query: literal by default, regex on a toggle, case: smart case, whole word option, glob include and exclude, scope (project, open files, changed files).

Engine v1 (own Go):

- Candidate files come from the Files index (no second walk). Skip binary (NUL byte in the first 8 KB) and files above a cap (for example 10 MB). Order candidates: open and recently edited files first, Git-modified files next, then the rest.
- Workers: `GOMAXPROCS-1`. Each worker memory-maps the file (`syscall.Mmap`) or reads it, runs `bytes.Index` in a loop (literal) or `regexp` (pattern), and emits per-file match groups.
- Unsaved editor buffers: search the rope text, not the disk (the rope is in `internal/rope`, so use the public buffer API).
- Streaming: emit results as files finish (like See and ripgrep). Stop at a global result cap (for example 1,000 lines) with a "more" cursor (file offset, like fff).
- Cancel: check `ctx.Done()` between files and every few MB inside big files.
- Memory: no index. Resident cost is near zero. Page cache does the caching.

Engine v2 (only if v1 is too slow on your targets):

- In-memory trigram index built lazily (idea from codesearch). Query → trigram set (for literals) or regexp-to-trigram query → candidate file IDs → verify with the v1 scan. Update per file when the watcher reports a change.
- Or bundle `rg`.

Files of 1 GB and more: the Text tab skips them by default (with a "skipped N large files" notice). In-file find is the editor's job (find bar on the rope).

### 7.7 Double Shift

- In `ObserveInput` on the main window: record `InputKeyDown` of `CodeShiftLeft` or `CodeShiftRight` with a timestamp. If another key arrives, reset. If a second Shift press arrives within a time window (start with 300 to 400 ms; make it a setting), open the panel.
- Open the panel as an overlay in the window (or a Panel window if you want it over other apps).
- Tab key switches tabs (All, Files, Actions, Symbols, Text). Direct shortcuts open a given tab. Query prefixes (`>`, `#`, `@`, `:line`) are optional shortcuts as in See.

## 8. Open points and unverified claims

- I did not run fff. All fff speed and memory numbers are from its README.
- I did not test purego struct reads on fff structs.
- The Go benchmark used 4,610 files. Scale numbers are linear estimates.
- JetBrains double Shift detection details are not checked.
- ripgrep license: GitHub lists "Unlicense"; the project is dual licensed (Unlicense or MIT). Check [the repository](https://github.com/BurntSushi/ripgrep) before you bundle it.
- The MyGo repository was read at its current default branch; the go.mod there says Go 1.27.3.
