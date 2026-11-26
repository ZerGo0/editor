# Large files (1 GB and more) in a MyGo-based editor

Date: 2026-11-26. MyGo commit: df8166243ec13c83bf1ca2216bacbd5a77b9efbf (shallow clone at <scratch>/clones/mygo). Go toolchain: go1.27.4 darwin/arm64, 10 CPU, 16 GB RAM.
Method: code reading plus one benchmark. Claims about other editors come from their docs or source. "Not verified" marks a claim without a primary source.

## 1. Summary

1. MyGo's buffer is a persistent rope. It is fast to edit at any size (about 1 microsecond per edit at 1 GB). It holds the whole text in Go heap memory as owned strings.
2. `editor.NewBuffer(text string)` needs the whole file as one Go string. Peak memory at 1 GB was 2.2 GB (Go runtime `Sys`), before the file read itself. Steady memory is 1.065 bytes per source byte.
3. `Buffer` is a concrete struct. `State.Buffer` is `*Buffer`. No interface, no lazy loader, no mmap path. Only `State.ReadOnly` exists.
4. Three editor features cost O(file size) on the main thread or per line: find (copies the whole text), line side tables (per line arrays), and highlighting from line 0 (line state chain). Diff is already off the main thread for large input.
5. Recommendation: use MyGo's buffer as is up to about 64 MB. Between 64 MB and 512 MB use it with heavy features off. Above 512 MB use a separate large-file mode: read-only first, pread block cache (or mmap), sparse background line index, piece table for edits.

## 2. How MyGo handles text (evidence from code)

### 2.1 Loading

- `NewBuffer(text string)` calls `newLineEnds(text)`, then `rope.New(t)`. See ui/editor/buffer.go (function NewBuffer) and ui/editor/lineends.go (newLineEnds, readEnds).
  https://github.com/egoist/mygo/blob/df8166243ec13c83bf1ca2216bacbd5a77b9efbf/ui/editor/buffer.go
- The caller must hold the whole file as a string first. There is no reader, no chunk source, no lazy path. `Reset(text string)` and `Set(text string)` also take a whole string.
- `readEnds` scans the whole string for `\r`. If the file has `\r`, it builds a second full copy with only `\n` (strings.Builder, Grow(len)). If not, it returns the same string. It also records one code byte per line break in a second rope (`lineEnds.codes`). Cost: one more full pass plus about 1 byte per line.
- `rope.Build` cuts the string in leaves of at most 4096 bytes (`ChunkBytes`) and clones each leaf with `strings.Clone`, so the rope owns its bytes and the input string can be freed. See internal/rope/rope.go (Build, leaf).
  https://github.com/egoist/mygo/blob/df8166243ec13c83bf1ca2216bacbd5a77b9efbf/internal/rope/rope.go
- Each node stores a Summary of bytes, runes, UTF-16 units, newlines. Line lookup is a tree walk by newline count (`LineStart`) and then `IndexByte` inside one leaf. So the line index is part of the tree. It is built during Build in one pass. No separate line table, no background build, no way to defer it.

### 2.2 Memory per byte and per line

Node struct: two child pointers (16 B), a string header (16 B), Summary of four ints (32 B), height (8 B) = 72 B. A tree of N leaves has about 2N nodes. So about 144 B per 4 KB leaf = 3.5 %. The line-ending rope adds 1 byte per line plus the same node overhead.
Measured (section 3): 1.065 bytes of heap per source byte at 1 GB. That is 6.5 % over the text. With 38 byte lines this is about 2.5 B of overhead per line.

Per line side tables in the editor (not in the buffer), from code:
- Highlighter: `states []uint64`, 8 B per line, in ui/editor/syntax.go (tokenHighlighter). It grows as the frontier moves.
- Soft wrap, when on: `extra []uint16` (2 B) plus Fenwick `tree []int32` (4 B) per line, in ui/editor/display.go (wrapRows.reset). Both are filled for the whole line count at once (`reset(n,...)` loops all lines).
- Not measured: height tables, fold state, line layout cache. The layout cache is per frame (view.go, `lines.m map[int]*lineLayout`), so it does not grow with the file.
So with highlight and wrap on, 26.7 million lines (1 GB test) cost about 14 B x 26.7 M = 370 MB more. Estimate from struct sizes, not measured.

### 2.3 Costs that grow with file size

| Area | What the code does | Growth | Evidence |
|---|---|---|---|
| Load | Whole string, one pass for `\r`, one rope build | O(size), synchronous in the caller | buffer.go NewBuffer |
| Line count | Kept in tree Summary | O(1) | rope.go Summary |
| Line lookup | Tree walk plus IndexByte in one 4 KB leaf | O(log n) | rope.go LineStart |
| Edit | Copies one leaf and the path to it | O(log n); 1 us at 1 GB | benchmark |
| Undo | Stack of inverse edits, `undoMax = 10000` steps; change log `logMax = 256` | Bounded by step count, not by size. One huge edit (Set, replace all) stores a copy of the replaced text | buffer.go |
| Highlighting | Line by line, state of line i-1 needed for line i. Budgeted per frame by time (`Highlight(last, budget)`). Lines over 10,000 bytes are left plain (`maxTokenize`) | To show line L, all lines before L must be tokenized once. A jump to the end of a 1 GB file needs about 27 M lines tokenized, in budget slices | syntax.go |
| Find | `ed.view.text.String()` copies the whole text. For case-insensitive ASCII search it makes a second full lowercase copy (`asciiLower`). Runs on the editor thread. Matches stop at `maxMatches = 100000` | O(size) memory and time per new query. Measured String() = 281 ms and 1 GB of garbage at 1 GB | find.go search, scan |
| Replace all | `textForReplacing` also uses `String()` | O(size) | find.go |
| Diff (git gutter, DiffView) | Full diff splits both texts into line slices (`ropeLines`). Runs on a goroutine above `diffSyncLines = 10000` lines or `diffSyncBytes = 1 MiB`, then edits update hunks locally | O(size) memory (a slice header per line, 16 B) and time | diffmodel.go full, lines |
| Buffer.changesBetween | Splits both texts in line slices | O(size) | buffer.go |
| Line endings | `MixedLineEndings` and `LineEnding` read counters | O(1). SetLineEnding converts a rope of codes | lineends.go |
| Save | `WriteTo` streams chunk by chunk. 220 ms for 1 GB to a discard writer | O(size), low memory | benchmark |
| `String()` | One full copy | O(size) | buffer.go |
| Soft wrap | Arrays for every line at once | O(lines) | display.go |

Docs claim: "Editors lay out and paint only the lines in view: a buffer of a million lines scrolls and edits as one of ten." (ui/editor/doc.go). This is true for paint and edit. It is not true for find, diff, soft wrap setup, and first highlight.
https://github.com/egoist/mygo/blob/df8166243ec13c83bf1ca2216bacbd5a77b9efbf/ui/editor/doc.go

### 2.4 Can a custom buffer be plugged in? Read-only or lazy path?

- No. `type Buffer struct` holds `text rope.Text` and many private fields (undo, log, syntax, ends). `State.Buffer *Buffer`. A search of ui/editor for interface types shows only `decor` (private). The rope package is under `internal/`, so an app outside the MyGo module cannot import it (Go internal rule).
- Read-only: `State.ReadOnly` makes text selectable and copyable but not editable (ui/editor/state.go). It does not change how the text is stored.
- Other knobs in State: `SoftWrap` (off by default), `LineNumbers` (`LineNumbersOff`), Language nil = plain text (no highlighter, `Buffer.highlighter()` returns nil). These give a "reduced" mode without a source change.
- Lazy load: none. mmap: none (search for mmap, ReadFile, os.Open in ui/editor shows no use).
- Conclusion: a large-file mode needs either a new widget (using MyGo's lower level `ui` drawing) or an upstream change that makes the text source an interface. MyGo is MIT, so a fork or patch is allowed.

## 3. Benchmark

Files: <scratch>/bench/main.go and go.mod (module "bench", `replace` points at the local clone of mygo). No window opened. Text: lines like `func f123() int { return 123 }` (about 38 bytes per line). `heap` = HeapAlloc after GC with the source string dropped. `Sys` = runtime total memory obtained from the OS after NewBuffer, with the 1 source string still alive.

| Case | NewBuffer | Lines | Heap after (bytes per source byte) | Sys after NewBuffer | Edit (mid file) | 1000 undo | WriteTo | String() |
|---|---|---|---|---|---|---|---|---|
| 100 MB, LF | 72 ms | 2,745,638 | 107 MB (1.070) | 221 MB | about 0.4 us each (447 us for 1000) | 1 ms | 6 ms | 10 ms |
| 100 MB, CRLF | 225 ms | 2,676,997 | 104 MB (1.042; CR removed from text, codes kept) | 334 MB | 0.6 us each | 1 ms | 43 ms | 6 ms |
| 1 GB, LF | 1.163 s | 26,730,832 | 1,091 MB (1.065) | 2,204 MB | 0.75 us each | 3 ms | 220 ms | 281 ms |

Other numbers: Line(mid) 3 to 6 us; Offset(mid) about 1 us; Modified() 6 to 9 us. TotalAlloc during gen plus NewBuffer was 3.78 GB at 1 GB (includes the 1 GB input). I did not find why NewBuffer allocates more than text plus rope; I did not profile it.
Meaning: the rope is cheap to edit and to save. The load is a one-time cost of about 1.2 s per GB plus the time to read the file into a string. The memory is the issue: about 1.1 GB steady, 2.2 GB peak, plus the string the caller read (so 2 to 3 GB for a 1 GB file), all anonymous memory (not reclaimable page cache).
Not measured (needs a window or unexported API): highlighter throughput, find, diff, layout. I derived those from code in section 2.3.
Side effect: `go mod tidy` downloaded go-text/typesetting, x/image, x/text, purego into the default Go module cache (outside the research folder).

## 4. How other editors handle huge files

| Editor | What it does | Source |
|---|---|---|
| VS Code | Large file optimizations decided once in the TextModel constructor. Too large for tokenization if text is over 20 MB or over 300,000 lines. "Too large for heap operation" over 256 M characters ("usually more than 512 MB of memory"). Sync to extension host stops at 50 MB. Local files up to 1024 MB open; above that it throws FILE_TOO_LARGE and shows "The file is not displayed in the text editor because it is very large". Remote 10 MB, web 50 MB. The decision is permanent for that model. Buffer is a piece tree (pieceTreeBase.ts), but the whole file is read into memory. | textModel.ts lines 188-191, 338-346, 368 (https://github.com/microsoft/vscode/blob/main/src/vs/editor/common/model/textModel.ts); files.ts lines 1672-1694 (https://github.com/microsoft/vscode/blob/main/src/vs/platform/files/common/files.ts); textFileEditor.ts lines 195-204 |
| Zed | Loads the whole file in memory. A temporary guard refuses files of 6 GB or more ("File is too large to load"). Comment in code: memory use is "in excess of 64GB for a 10GB file"; 5 GB peaks at about 16 GB. Open issues about freezes on large files and on search in large files exist. | worktree.rs line 1742-1754 (https://github.com/zed-industries/zed/blob/main/crates/worktree/src/worktree.rs); issues #27283, #61944 |
| Sublime Text | No hard limit. Whole file is in memory; users report about 1 GB RAM for a 1 GB file and about 10 GB RAM for a 5 GB file (45 M lines), 1 to 2 minutes to load, with a progress bar and abort. A user quotes wbond: "there isn't a way to detach the display of the file with the in-memory representation". Plain Text syntax is much faster (no regex parse). Word wrap needs a check of every line. No official doc found; this is a forum thread, so weak evidence. | https://forum.sublimetext.com/t/sublime-text-performance-with-very-large-files/9832 |
| JetBrains IDEs | Content load limit 20,000 KB (about 20 MB) by default (`idea.max.content.load.filesize`); coding assistance limit 2,500 KB (`idea.max.intellisense.filesize`). Both are settings. This is the reference for "JetBrains like" behavior. | https://www.jetbrains.com/help/idea/tuning-the-ide.html |
| EmEditor | 64-bit: up to 16 TB or 1,099 billion lines. Above a threshold (default 300 MB) it uses temporary disk space instead of memory. Above that size, multi-line comment highlighting and wrap modes are disabled. Multithreaded: you can view the file while it opens; cancel possible. Beyond 16 TB the Large File Controller opens a part (start, end, range). | https://www.emeditor.com/text-editor-features/large-file-support/files-up-to-248gb/ |
| UltraEdit | Text files over 10 GB (a 35 GB demo). Disk based: parts read on demand; search runs on disk. By default it may make a temp copy; "Open file without a temp file" edits the original directly (suggested threshold 50 MB = 51,200 KB). Large File Mode cuts line numbers, code folding, syntax highlighting. Line change indicator and line numbers cost time. | https://www.ultraedit.com/support/tutorials-power-tips/ultraedit/large-file-handling/ |
| 010 Editor | "Data engine": data is "not read into the editor until it is required"; blocks point to disk or memory; unlimited undo and redo; "no limit to the size of files that can be opened". Clipboard copies over 16 KB may store pointers, so data can be corrupted if the source file changes on disk (the manual warns about this). On text files, large blocks are scanned for line feeds (a delay). | https://www.sweetscape.com/010editor/manual/DataEngine.htm; Wikipedia summary not used as proof |
| klogg (glogg fork), read-only log viewer | Background indexing thread reads the file in 5 MB blocks (`IndexingBlockSize = 5 * 1024 * 1024`) with `QFile::read`, not mmap. Stores end-of-line offsets (an option for a compressed index exists). Shows lines on demand by offset. Detects change on disk by hashing the indexed part, or only the first and last blocks (fast mode, can miss middle changes); distinguishes append (continue indexing, "follow" mode) from overwrite/truncate (clear results). | logdataworker.cpp lines 69, 510-531; DOCUMENTATION.md lines ~236-250, 364-374 (https://github.com/variar/klogg/blob/master/DOCUMENTATION.md) |

Techniques seen across these tools:
- Whole file in memory: VS Code, Zed, Sublime. They survive by turning features off (VS Code) or by a hard cap (Zed, VS Code 1 GB local).
- Disk backed: EmEditor (temp file), UltraEdit (direct file, on disk search), 010 Editor (block table with pointers into the file), klogg (offset index plus reads).
- Background line index: klogg, and EmEditor (view while opening). 010 Editor scans text for line feeds when blocks are opened.
- Features disabled above size: VS Code (tokenization, heap operations), EmEditor (multi-line highlight, wrap), UltraEdit (folding, highlighting, line numbers), JetBrains (coding assistance at 2.5 MB).
- I found no primary source on hecate-like hex viewers; the 010 Editor data engine is the closest design (block table over a file).

### 4.1 mmap hazards (file changes or shrinks while mapped)

- The Rust memmap2 docs say file backed maps are unsafe because of "the potential for Undefined Behavior (UB) using the map if the underlying file is subsequently modified", "in or out of process". https://docs.rs/memmap2/latest/memmap2/struct.Mmap.html
- In POSIX, reading a mapped page past the new end of a truncated file raises SIGBUS (general OS behavior; I did not fetch a man page, so not verified here).
- In Go, a fault at an unexpected address crashes the process by default. `runtime/debug.SetPanicOnFault` turns it into a recoverable panic, but "applies only to the current goroutine". So every goroutine that reads mapped memory must enable it and recover. https://pkg.go.dev/runtime/debug#SetPanicOnFault
- `golang.org/x/exp/mmap` gives a read-only `ReaderAt` over a mapping. https://pkg.go.dev/golang.org/x/exp/mmap
- Editors that write via "write temp file, then rename" give the file a new inode. An old fd or old mapping then keeps the old content, which is safe. Editors that write in place or truncate (logs, `> file`) are the dangerous case.
- Safe alternative: positional reads (`pread`, `os.File.ReadAt`) with an own block cache. A short read gives EOF, not a crash. klogg uses this. Cost: copy per read; cache memory is under your control and is bounded.

## 5. Recommended design

### 5.1 Tiers and thresholds

Thresholds are proposals. They come from the sources above and from the benchmark; tune them with a benchmark of your own highlighter and find on your target Mac.

| Tier | File size and shape | Storage | Features |
|---|---|---|---|
| Normal | up to 20 MB and up to 300,000 lines | MyGo `Buffer` as is | All. Same size as VS Code's and JetBrains' limits for tokenization and coding assistance (20 MB), which users already know. |
| Reduced | over 20 MB up to 512 MB | MyGo `Buffer`, loaded on a goroutine, not on the UI thread | Language nil (no highlight), `SoftWrap` off, git gutter diff off, no replace all through `String()`, no DiffView, find through your own streaming search (section 5.3). Edit, undo, multi-cursor stay on. Memory about 1.07 x size plus the read string. At 512 MB that is about 0.55 GB steady and 1.1 GB peak (extrapolated from the benchmark). |
| Large | over 512 MB, or over 1/8 of installed RAM, or over 5 million lines, whichever comes first (any file where the Reduced tier would take more than about 1 s to load) | Separate large-file mode (section 5.2) | Section 5.3 |
| Hard stop | more than free disk space needed for save, or a non-regular file (pipe, device) | none | Read-only viewer only |

Why 20 MB: both VS Code (20 MB, 300 K lines, textModel.ts) and JetBrains (20,000 KB content limit) use it. Why 512 MB: the benchmark shows the rope load takes 1.2 s and 2.2 GB peak per GB. At 512 MB it is about 0.6 s and 1.1 GB peak. This is also close to VS Code's "heap operation" limit (256 M characters, about 512 MB of memory). Above it, the memory is anonymous, not file cache, and breaks the low memory goal. Why the RAM rule: Zed and Sublime show memory blowing to 2x to 6x file size on large input (Zed comment, Sublime thread).
Do the mode choice by stat size before reading the file, and allow "Open in large mode" / "Open normally" by the user (JetBrains lets users raise limits). VS Code makes the choice once per model and keeps it (textModel.ts comment: "permanently respect this decision"); do the same, or the UI state gets complicated.

### 5.2 Large-file mode: data structures

1. Source: read-only file handle. Default: pread with a block cache (for example 256 KB blocks, LRU, 64 to 128 MB cap). Optional later: mmap with `SetPanicOnFault` plus recover on each reader goroutine. Reason: a truncate by another process cannot crash the app with pread. The tradeoff is one copy per block read.
2. Piece table for edits. A balanced tree (or B-tree) of pieces. A piece = (source: original file or add buffer, offset, length, newline count). Summaries in tree nodes: bytes and newlines (as MyGo's rope Summary does). Start: one piece for the whole file. Edits split pieces and append text to an append-only add buffer in memory. Memory is proportional to the number of edits, not to the file size. This is the 010 Editor data engine and the VS Code piece tree design. Undo = list of edit records (offset, removed piece list, inserted piece list), capped by count and by add buffer bytes (for example 10,000 steps like MyGo, and 64 MB of add buffer).
   Newline counts of a split piece need a scan of that piece range only, using the sparse line index (below) and `bytes.Count` on the remainder. Cap the size of a single read for a split (scan at most one block each side).
3. Line index: sparse, built by a background goroutine. Scan blocks of 4 to 8 MB (klogg uses 5 MB) with `bytes.IndexByte`/`bytes.Count` (SIMD in the Go standard library on arm64). Store the byte offset of every 1,024th line (for 27 M lines: 26 K entries, about 200 KB). Line N lookup = binary search, then scan at most 1,024 lines (about 40 KB). A full per-line offset table would cost 8 B x 27 M = 214 MB; klogg offers a compressed index for the same reason. Until the scan ends, navigate by byte offset: the scrollbar maps to bytes, the status bar shows "indexing 63 %", line numbers show "~". The scan also records: line count, longest line length, line ending counts (LF, CRLF, CR), UTF-8 validity. Expected speed: memory bandwidth bound after the file is in page cache (the benchmark's WriteTo shows 1 GB in 220 ms for rope chunks; a cold disk read is limited by the SSD).
4. Viewport: paint only visible lines. For each visible line, read its bytes (cache hit), cut at a layout limit (for example 10,000 bytes, as MyGo's `maxTokenize`; show the rest through horizontal paging), then shape and paint with MyGo's UI text drawing. Draw with no state kept for off-screen lines. Cache shaped lines for the last frame only (as MyGo does).
5. Selection and caret: a small set of (offset) values in bytes, not line/col. Keep multi-cursor to a limit (for example 1,000).
6. Save: stream pieces to a temp file in the same directory, fsync, then rename over the original (new inode; old readers keep the old content), then reopen the file and reset the piece table to one piece (clear undo, or keep a flag to warn). Needs free space equal to the file size. Check free space before the write. Keep file mode, owner, and xattrs if you want parity with other editors (not researched).
7. Optional: promote to the MyGo `Buffer` for a range ("Open this 50 MB slice in the normal editor"), like EmEditor's Large File Controller opens a part of a file.

### 5.3 Large-file mode: threads and features

Threads (goroutines) and rules:
- UI goroutine: input, paint, piece table edits. It never reads more than a few blocks per frame. A cache miss on a block shows a placeholder line and triggers a read on a worker (or a read of one block, which takes well under 1 ms from the page cache). Not verified: SSD latency on your target; measure it.
- Index worker: one goroutine, low priority (call `runtime.Gosched` between blocks, or use one block per tick), cancellable by context, reports progress by an atomic counter. Redo from the changed point when the file changes.
- Search worker(s): stream blocks of the file plus the piece table, with an overlap of (pattern length - 1) bytes between blocks. Literal search with `bytes.Index` (fast). Regex: Go's `regexp` is slower; run it per line and tell the user. Stop at 100,000 hits like MyGo's `maxMatches`. Show hits as they arrive. Cancel on a new query. Do not copy the whole text (this is what MyGo's find does).
- File watcher: kqueue through MyGo's watch (a `watch.go` exists in the repo root; I did not read it) or fsnotify. On event, or on window focus, `stat` (size, mtime, inode). Apply klogg's logic: if size grew and the first and last indexed blocks hash the same, treat as append (continue index, optional follow mode like `tail -f`). If size shrank, inode changed, or the hash differs: show a non-modal banner "File changed on disk" with Reload / Keep. Never auto-discard user edits. With pread the view stays safe (EOF on short read); with mmap this check must run before every map access or the process can get SIGBUS.
- Git (JetBrains-like): use `git diff` hunks computed in a background process for files over the Reduced limit only if you want gutter marks; it is an optional feature. Large mode: no gutter marks, no DiffView in the editor (MyGo's diff splits both texts into per line slices, about 16 B x lines plus the text). This is a recommendation, not measured.

Feature switches in Large mode:
- Off: syntax highlighting (MyGo's tokenizer chain needs all lines before the visible line; VS Code and EmEditor also turn it off), soft wrap, folding, minimap, bracket matching outside the window, git gutter, replace all (offer a streamed "replace all into a new file" instead), line ending conversion on the whole file, spell check.
- Window highlighting (optional, later): lex only the visible lines with state 0 at the window start. It can show wrong colors inside block comments and strings that start above the window. Mark as "approximate". Not found in any source; this is an idea.
- On: scroll, go to line (needs index; go to byte offset any time), find (streamed), copy, select, edit with piece table, undo, save.
- Line numbers: on after the index finishes (UltraEdit says line numbers slow navigation for huge files; with a sparse index it is cheap).
- Encoding: UTF-8 only in v1. Detect with the index scan; offer "Open as Latin-1" only if needed. Not researched.

### 5.4 Order of work

1. Reduced tier with MyGo as is: load on a goroutine (read file, then `NewBuffer`), progress UI, Language nil, `SoftWrap` off, own streaming find. No MyGo change. Measure the real editor (highlighter, find, frame time) at 100 MB and 500 MB with a window; this research did not.
2. Large mode, read-only: pread block cache, sparse line index, viewport painter, streamed search, change detection. Gives a fast log and data viewer.
3. Large mode editing: piece table, undo, save by temp and rename.
4. Optional upstream work in MyGo: a text source interface for `ui/editor` so the same view works with both buffers. Weigh this against maintaining a fork.

## 6. Risks and open points

- Not measured: highlighter speed, real find cost, layout, memory of the editor view (needs a window; the rules forbid opening one). Numbers in 2.2 and 2.3 for these are from code, not from a run.
- The benchmark uses 38 byte lines. Files with very short lines (more lines per byte) raise the per-line costs; files with very long lines (minified JSON) hit the 10,000 byte limit in MyGo's tokenizer and layout.
- The 20 MB, 512 MB, and 1/8 RAM thresholds are proposals. VS Code, JetBrains, EmEditor, and UltraEdit use 20 MB, 20 MB, 300 MB, and 50 MB as limits for different features, so there is no single standard.
- Sources for Sublime Text are forum posts, not official docs. EmEditor and UltraEdit pages are marketing and help pages; they do not state line index internals.
- MyGo has no license problem (MIT). See (wibus-wee/see) was not read.
