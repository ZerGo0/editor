# What developers like and dislike in VS Code and JetBrains IDEs, and what it means for our editor

Date of research: 2026-11-26. Focus: polish and the feel of the tool.

## 0. Method, evidence and limits

- Hard numbers come from public APIs that I read myself: the GitHub issue search for microsoft/vscode (open issues sorted by +1 reactions), the JetBrains YouTrack API (open issues sorted by votes), the VS Code Marketplace query API (sorted by installs), and the Hacker News Algolia API (comments read in full).
- Reddit blocked automated reads (HTTP "Blocked"). Search did not return r/vscode, r/Jetbrains or r/golang threads. This report has no Reddit quotes. Hacker News, DEV Community and issue trackers replace them. Treat Reddit as an open gap.
- Marketplace install counts include automatic dependencies (for example the Jupyter family and the Java pack), so they overstate direct demand. I use them only for tools that people choose by hand (Prettier, GitLens, Error Lens and similar).
- Section 4 (polish details) comes from product documentation and long use of both tools. The links go to the docs. I did not test each behavior in this session.
- Vendor numbers (JetBrains "95 percent say IntelliJ IDEA improves productivity") are marketing claims. I mark them as such.

### Code check: what our editor really has

I read docs/plan.md (no README exists), listed internal/, and read the MyGo editor widget (github.com/egoist/mygo v0.10.0, ui/editor). Corrections to the "not planned yet" list:

| Item | Result of the check |
|---|---|
| Code folding UI | EXISTS. Widget commands Fold, Unfold, FoldAll, FoldLevel1 to 9 are registered as commands. Setting editor.folding is in internal/settings/schema.go. Remove it from the missing list. |
| Multi-cursor | EXISTS. AddCursorAbove, AddCursorBelow, SelectNext, SelectAllMatches, SplitSelectionIntoLines. |
| Split views | EXISTS (internal/layout, split commands). |
| Snippets | PARTLY. The widget has a snippet engine with tab stops (ui/editor/snippet.go). The app has no user snippets and no built-in snippets. Only issue and user completions (# and @) use completions. |
| Completion, hover, go to definition, diagnostics, inlay hints | WIDGET HOOKS EXIST (OnCompletion, OnHover, OnDefinition, SetDiagnostics, SetInlays, SetSuggestion). The app wires only: SetDiagnostics (bracket, JSON, YAML, EOL and conflict checks in internal/problems) and SetInlays (inline blame). No language intelligence. |
| Outline view | NOT AS A VIEW. "Go to Symbol in Editor" and breadcrumb symbols exist. They use a regex symbol index (internal/search/symbols). |
| Integrated terminal | MISSING. Only "Open in Terminal", which starts Terminal.app. |
| Minimap, sticky scroll, bracket pair colors | MISSING. Bracket matching (jump and highlight) exists. The widget notes a minimap use for its viewport data. |
| Formatting on save | PARTLY. Trim trailing whitespace and insert final newline exist. No formatter. |
| Hot exit | MISSING. internal/session stores path, cursor, scroll and pinned state, not unsaved text. |
| Word wrap, rulers, indent guides, render whitespace, auto close brackets, auto save, line numbers (relative too), cursor style, zoom | EXIST as settings. |
| Toggle comment, sort lines, case changes, move and duplicate lines, join lines, transpose | EXIST as widget commands, registered in the command palette. |
| Status bar item hide or show, language and EOL and indentation pickers, tooltips with key names | EXIST. |
| Color themes, folder settings (.vscode), changelists, scratch files, back/forward navigation, EditorConfig, compact folders, spell check, zen mode | MISSING. |
| Dark and light theme follow | UNVERIFIED. No theme setting found. |

Plan decisions that conflict with the top user demands: "No LSP", "No terminal, no run tasks", "No plugins". Section 6 shows cheaper steps inside those limits.

## 1. Top reasons people love VS Code (ranked)

1. **One free tool for every language, quick to open.** 75.9 percent of 26,143 respondents use it, the top IDE five years in a row ([Stack Overflow Developer Survey 2025](https://survey.stackoverflow.co/2025/technology), [Visual Studio Magazine summary](https://visualstudiomagazine.com/articles/2025/08/01/stack-overflow-dev-survey-visual-studio-vs-code-hold-of-ai-ides-to-remain-on-top.aspx)). HN commenters describe a common pattern: VS Code for everything unless JetBrains is needed (summary of [HN 26367653](https://news.ycombinator.com/item?id=26367653) and [HN 36413457](https://news.ycombinator.com/item?id=36413457)). One user: "type 'code .' and do some quick editing" without making a project (HN 36413457).
2. **The extension marketplace. People add what the core lacks.** Top hand-picked installs: Prettier 73.2M, GitLens 54.1M, ESLint 54.0M, Live Server 84.8M, Code Runner 42.9M, Material Icon Theme 36.3M, Error Lens 10.4M, Todo Tree 7.9M, Vim 9.2M, Git Graph 15.8M ([Marketplace by installs](https://marketplace.visualstudio.com/search?target=VSCode&category=All%20categories&sortBy=Installs), [GitLens](https://marketplace.visualstudio.com/items?itemName=eamodio.gitlens), [Prettier](https://marketplace.visualstudio.com/items?itemName=esbenp.prettier-vscode)). Each one marks a gap: formatter, Git depth, linter, run button, icons, inline errors.
3. **Remote work: containers, SSH, WSL.** Dev Containers 42.9M, SSH 38.9M, WSL 41.4M installs. HN: "one killer feature of VSCode is the ability to connect to WSL, remote server and Docker container" ([HN 26367410](https://news.ycombinator.com/item?id=26367410)); "They [JetBrains] don't do containerized development well" ([HN 36413457](https://news.ycombinator.com/item?id=36413457)).
4. **Easy to configure.** settings.json plus a Settings UI, Settings Sync, Profiles. HN commenters call it far easier to configure, with many more plugins ([HN 26367653](https://news.ycombinator.com/item?id=26367653)), and praise Settings Sync (an HN comment found through web search; the exact thread is not pinned). Docs: [Settings Sync](https://code.visualstudio.com/docs/editor/settings-sync).
5. **Command palette and keyboard-first flow.** A user asks for more teaching of shortcuts: [#26729](https://github.com/microsoft/vscode/issues/26729) has 429 +1. A JetBrains user wants JetBrains to copy VS Code's search: "PyCharm search and find is so awkward, and VS Code's is so good" ([HN 29887706](https://news.ycombinator.com/item?id=29887706)).
6. **Lighter than a full IDE.** HN: "VS Code does well with performance" next to JetBrains, Visual Studio, Eclipse and NetBeans ([HN 45972959](https://news.ycombinator.com/item?id=45972959)). The claim is relative and fades with many extensions (section 3).
7. **Good-enough built-in Git, terminal and debugging for common languages, plus a fast release rhythm.** [An Honest Comparison of VS Code vs. JetBrains](https://dev.to/blankly/an-honest-comparison-of-vs-code-vs-jetbrains-5-points-1mbd): conflict resolution inline with one click, debugging with little setup for Python and JS.
8. **First place for AI tools.** Copilot 75.0M and Copilot Chat 79.5M installs. Cursor, a VS Code fork, has 17.9 percent usage in the Stack Overflow survey. Users take the VS Code shell and add AI.

## 2. Top reasons people love JetBrains IDEs (ranked)

1. **Code intelligence and refactoring that "just work".** HN: "Every IDE function appears to be better implemented in JetBrains, whether it's Code Analysis, Refactoring, Navigation, Running/Debugging, Running Tests, adding new files, git integration" ([HN 26367410](https://news.ycombinator.com/item?id=26367410)). "For typed languages like Java and Kotlin there is really no competition." Also [Why I switched from VS Code to WebStorm](https://dev.to/mokkapps/why-i-switched-from-visual-studio-code-to-jetbrains-webstorm-939): unused-code detection, safe delete, rename that updates references.
2. **It works out of the box and the parts fit together.** HN: "JetBrains comes with all of that out of the box and playing nicely with each other"; plugin management is "much easier" ([HN 29887706](https://news.ycombinator.com/item?id=29887706)).
3. **Debugger, run configurations and test runner.** HN: "the testing/debugging setup just works too good out of the box"; "Debugging features are not even comparable, the difference is day and night" ([HN 29887706](https://news.ycombinator.com/item?id=29887706), [HN 26367410](https://news.ycombinator.com/item?id=26367410)). Rerun one failed test in a click (mokkapps).
4. **Git and history tooling.** "The side by side by side merge conflict resolution is so good that even if JetBrains had a product that did only that, I would use it" ([HN 29887706](https://news.ycombinator.com/item?id=29887706)). The most-voted open JetBrains platform request is a standalone Git client: [IJPL-72504](https://youtrack.jetbrains.com/issue/IJPL-72504), 702 votes. 242 users argued to keep the modal commit window: [IJPL-177161](https://youtrack.jetbrains.com/issue/IJPL-177161). Local History: "The only feature I miss is the local history" ([HN 26367410](https://news.ycombinator.com/item?id=26367410)).
5. **Language depth in each product.** GoLand: "The refactorings are generally smarter than what's implemented in gopls" ([HN 29887706](https://news.ycombinator.com/item?id=29887706)). VS Code Go "breaks in unexpected ways after every update" ([HN 26367410](https://news.ycombinator.com/item?id=26367410)). RubyMine "actually get[s] Rails projects".
6. **Search and navigation across big code.** Search Everywhere, Go to Symbol, Find Usages. Evidence is mixed: DEV Community commenters call it the best feature ([article](https://dev.to/ramazanimwemedi/why-jetbrains-ides-are-better-than-vs-code-for-developers-511p)); one HN user finds the Find dialog "awkward".
7. **Customizable UI and consistent keymap.** HN: "UI customizability is miles ahead in JetBrains" ([HN 29887706](https://news.ycombinator.com/item?id=29887706)).
8. **Satisfaction data (vendor).** 53 percent of PhpStorm users give the top rating vs 26 percent of VS Code users, and 95 percent of Java developers say IntelliJ IDEA helps productivity ([JetBrains State of Developer Ecosystem 2025](https://blog.jetbrains.com/research/2025/10/state-of-developer-ecosystem-2025/) and its language reports). Vendor numbers. Stack Overflow shows IntelliJ IDEA at 27.1 percent, PyCharm at 15 percent.

## 3. Top complaints about each, and what we can do better

### VS Code

| Complaint | Evidence | What we can do better |
|---|---|---|
| Memory and CPU grow with extensions and time | [#148226](https://github.com/microsoft/vscode/issues/148226) renderer at 100 percent CPU; [#165016](https://github.com/microsoft/vscode/issues/165016) laggy after use on macOS; [#107016](https://github.com/microsoft/vscode/issues/107016) slow scroll and typing; an HN comment (found through web search, thread not pinned): over 1 GB to open a few small files vs about 300 MB for Sublime | Native app. Keep idle at zero frames (already planned). Publish budgets. |
| Core features missing, so you need extensions | Formatter, Git graph, run button, inline errors all appear in the top installs (section 1) | Build the top 10 extension features in. Start with format, Git graph, TODO, inline errors, icons. |
| Extension breakage and uneven quality | A PhpStorm-to-VS Code switcher reports that Prettier, auto-imports and go-to-definition kept breaking (DEV Community posts found through search, for example [this one](https://dev.to/dailydevtips1/i-stopped-using-visual-studio-code-1b81/comments)). HN: "gopls has not helped at all" ([HN 26367410](https://news.ycombinator.com/item?id=26367410)) | No plugin system means fewer broken combinations. One tested path per feature. |
| Basic Git | [#179053](https://github.com/microsoft/vscode/issues/179053) graph, 612 +1; [#249974](https://github.com/microsoft/vscode/issues/249974) changelists; [#183048](https://github.com/microsoft/vscode/issues/183048) shelve; [#36893](https://github.com/microsoft/vscode/issues/36893) compare with any commit | In progress. This is our main edge. |
| Project-wide problems only for open files | [#13953](https://github.com/microsoft/vscode/issues/13953), 1511 +1 | Problems panel covers the project, not only open tabs, when a checker exists. |
| Settings sprawl and weak scoping | [#15909](https://github.com/microsoft/vscode/issues/15909) extend settings files, 1375 +1; [#5595](https://github.com/microsoft/vscode/issues/5595) per platform, 1152 +1; [#40233](https://github.com/microsoft/vscode/issues/40233) local workspace settings, 1048 +1 | Few, well-named settings. Show "modified" marks. Add folder settings later. |
| Limited UI customization | [#519](https://github.com/microsoft/vscode/issues/519) workbench font size, 3808 +1 (our zoom setting covers part); [#3130](https://github.com/microsoft/vscode/issues/3130) mouse shortcuts, 1938 +1; [#158603](https://github.com/microsoft/vscode/issues/158603) sidebar widths | Remember sidebar and panel sizes per project. Keep UI zoom. |
| Telemetry and AI clutter | [#176269](https://github.com/microsoft/vscode/issues/176269) consent before telemetry, 1005 +1; an HN commenter says they dropped VS Code when AI and complexity grew (found through web search, thread not pinned) | No telemetry. No AI nags. Say so. |
| Tabs | [#100335](https://github.com/microsoft/vscode/issues/100335) grouped tabs, 858 +1; [#108264](https://github.com/microsoft/vscode/issues/108264) vertical tabs, 663 +1 | Low priority. Preview and pinned tabs exist. |

### JetBrains

| Complaint | Evidence | What we can do better |
|---|---|---|
| Slow, heavy, high CPU | HN: "tools are clunky resource hogs"; "Typing a character spins up several cores at 100 percent" ([HN 48185188](https://news.ycombinator.com/item?id=48185188)); [PY-81030](https://youtrack.jetbrains.com/issue/PY-81030) stuck editing, high CPU; [WEB-52943](https://youtrack.jetbrains.com/issue/WEB-52943) high CPU in TypeScript; [WEB-67306](https://youtrack.jetbrains.com/issue/WEB-67306) slow completion; [PY-80871](https://youtrack.jetbrains.com/issue/PY-80871) 2025.1 regressions | Budgets for typing latency, scroll and idle CPU in the E2E suite. Fail the build on regression. |
| Indexing and re-indexing with no reason | HN: "I cannot for the life of me understand why it keeps re-indexing" and "a UI, or even a text log, that says I'm reindexing because X" ([HN 48185188](https://news.ycombinator.com/item?id=48185188)) | Show every background job with its reason and a Cancel button. |
| Slow startup, heavy project setup | HN: "Jetbrains process of creating a project around the directory" ([HN 36413457](https://news.ycombinator.com/item?id=36413457)); [IJPL-8382](https://youtrack.jetbrains.com/issue/IJPL-8382) keep .idea outside the project, 313 votes | Open any folder at once. Write no project files into the folder. |
| Modal dialogs and blocking UI | [IJPL-43713](https://youtrack.jetbrains.com/issue/IJPL-43713) per-project modality, 371 votes; [IJPL-338](https://youtrack.jetbrains.com/issue/IJPL-338) redesign file chooser, 329; [IJPL-99530](https://youtrack.jetbrains.com/issue/IJPL-99530) non-modal merge, 196 | Use sheets and popovers. Never block other windows. Keep the commit window as a view, with the option to dock it. |
| Ads, trial buttons, AI pushes | [IJPL-222323](https://youtrack.jetbrains.com/issue/IJPL-222323) "Start Free Trial" cannot be removed, 225 votes; HN: "I had my breakup when they started advertising their code assistant at me" ([HN 48185188](https://news.ycombinator.com/item?id=48185188)) | None. A tool is a home, not a billboard. |
| Keyboard layouts | [IJPL-54496](https://youtrack.jetbrains.com/issue/IJPL-54496) national layouts, 530 votes | Match shortcuts by key code. Test with non-Latin layouts. |
| UI churn and contrast | [IJPL-55546](https://youtrack.jetbrains.com/issue/IJPL-55546) new UI light theme contrast, 72; [IJPL-996](https://youtrack.jetbrains.com/issue/IJPL-996) gutter too wide, 118 | Check contrast in both modes. Keep the gutter narrow. |
| Project tree lacks VCS info and sticky headers | [IJPL-73102](https://youtrack.jetbrains.com/issue/IJPL-73102) branch in Project View, 281; [IJPL-48492](https://youtrack.jetbrains.com/issue/IJPL-48492) sticky headers in tree, 134 | Show the branch in the sidebar header. Pin parent folders in the tree. |
| Cost and the dead lightweight option | JetBrains stopped Fleet on 2025-12-22 ([InfoWorld](https://www.infoworld.com/article/4104991/jetbrains-discontinues-fleet-ide.html), [DevClass](https://www.devclass.com/ai-ml/2025/12/09/jetbrains-abandons-fleet-ide-pins-hopes-on-forthcoming-air-agentic-development-tool/1728894)). HN on Fleet in 2021: "DOA without vim bindings", "starts up in seconds. Shame." ([HN 29377515](https://news.ycombinator.com/item?id=29377515)) | The lightweight, fast, Git-strong editor slot is open. |
| Remote latency | HN: JetBrains remote "couldn't keep up" on a slow link ([HN 48185188](https://news.ycombinator.com/item?id=48185188)) | Not our scope. |

### Newer editors: what they teach (polish and speed)

- **Zed.** Praised for low latency and a GPU renderer: "good lord is the thing fast" ([HN 43912844](https://news.ycombinator.com/item?id=43912844)); reviews note under one second cold start. Complaints: blurry text on 1440p ([zed #7992](https://github.com/zed-industries/zed/issues/7992)), 130 percent CPU on Clojure files, a sign-in button that cannot be removed. Lesson: text rendering and idle CPU make or break trust.
- **Sublime Text.** "Love how lightweight and fast it is"; "I'm happy with just fast, multiple cursors, plugins" ([HN 42862246](https://news.ycombinator.com/item?id=42862246)). It opens faster than VS Code ([HN 27230042](https://news.ycombinator.com/item?id=27230042)). Gaps: no integrated terminal, laggy Vim plugins (same thread). Lesson: instant open and big-file speed buy loyalty for years.
- **Cursor.** Adopted for AI. A user "lost code" because undo got hard to follow in a review of an agent edit ([HN 43912844](https://news.ycombinator.com/item?id=43912844)). Lesson: trust in undo and history matters more than new features. Our Local History helps.
- **Nova (Panic).** Praised for native feel and low resource use; "Panic's level of polish is unrivaled" ([HN 30066298](https://news.ycombinator.com/item?id=30066298), [review](https://tracks.ranea.org/post/629525278016798720/panics-nova-text-editor-a-review)). Criticized for slowness on 10,000-file projects and for a non-standard title bar, tabs and outline. Lesson: native feel is a promise. Test with a 100,000-file project (already in the plan).

## 4. Polish details that make each tool feel good

Source note: product documentation and long use. Links are to docs.

### VS Code

- **One input, many modes.** Cmd+P with prefixes: `>` commands, `@` symbols, `:` line, `#` workspace symbols, `?` help. Fuzzy matches are underlined in the row. Recent files come first. Arrow keys preview without opening. [Docs](https://code.visualstudio.com/docs/editor/userinterface#_command-palette).
- **Preview tabs.** Italic title, replaced by the next single click, kept by edit or double-click. Dirty dot replaces the close button. [Docs](https://code.visualstudio.com/docs/editor/userinterface#_working-with-editors).
- **Breadcrumbs with a dropdown tree** of the symbols in the file. Sticky scroll pins the current class and function lines. Bracket pair colors and guides. [Docs](https://code.visualstudio.com/docs/editor/editingevolved).
- **Settings UI.** Search at the top, blue bar on modified settings, a gear menu with "Copy Setting ID" and "Reset", scope tabs User and Workspace, and a link to the JSON. [Docs](https://code.visualstudio.com/docs/getstarted/settings).
- **Inline diff peek in the gutter** with revert arrows. Timeline view per file. Diff editor with inline mode. [Docs](https://code.visualstudio.com/docs/sourcecontrol/overview).
- **Smooth cursor and scroll options.** editor.smoothScrolling and editor.cursorSmoothCaretAnimation. Folding arrows show only on hover.
- **Peek Definition** in a small inline window. Hover cards with docs. Light bulb for quick fixes.
- **Drag and drop with drop overlays.** Drag a tab to an edge to split. Drop a file into the editor to open it.
- **Notifications with progress and "Don't show again"**, plus status bar progress.
- **Terminal shell integration.** Command marks in the scrollbar and "run recent command" (HN 36413457: "side markers to help scrolling back").
- **Welcome walkthroughs and a watermark with shortcuts** in the empty editor. [Docs](https://code.visualstudio.com/docs/getstarted/getting-started).

### JetBrains

- **Search Everywhere (double Shift)** with tabs and a preview. Find Action shows the toggle state and also finds settings. [Docs](https://www.jetbrains.com/help/idea/searching-everywhere.html).
- **Recent Files (Cmd+E) and Recent Locations (Cmd+Shift+E)** with a preview pane. Back and forward (Cmd+[ and Cmd+]). [Docs](https://www.jetbrains.com/help/idea/navigating-through-the-source-code.html).
- **Alt+Enter on anything.** Quick fixes and intentions in one popup. Reformat (Cmd+Opt+L), Extend Selection (Opt+Up), Surround With, postfix completion (`.if`, `.for`). [Docs](https://www.jetbrains.com/help/idea/intention-actions.html).
- **Popups close with Esc and tool windows hide with Shift+Esc.** Focus goes back to the editor. Cmd+1 shows and hides the project tree.
- **Local History with labels, and the commit window with changelists, checks and message history.** [Local History](https://www.jetbrains.com/help/idea/local-history.html), [Changelists](https://www.jetbrains.com/help/idea/managing-changelists.html).
- **Gutter run icons** next to main functions and tests, plus inline values while debugging. Inlay hints for parameter names.
- **Background tasks indicator** in the status bar, with a list and Cancel. Power Save Mode.
- **Scratch files, bookmarks with mnemonics, and Autoscroll from Source** for the project tree. [Scratches](https://www.jetbrains.com/help/idea/scratches.html).
- **Productivity Guide and Tip of the Day** teach shortcuts in context.

### What this means for the "feel"

1. Fast first paint, instant open, no frames at idle: Zed, Sublime and Nova win loyalty with this alone.
2. One input for many jobs (palette prefixes, Search Everywhere) and a visible keyboard hint on everything.
3. Never block the user: popovers and sheets, not modal windows. Esc always backs out.
4. Trust: unsaved text survives (hot exit), history is easy, undo is clear.
5. Say what is running and why (indexing, Git, search), with a Cancel button.

## 5. Gap table

Value is high, med or low user value. Effort: S is days, M is about 1 to 2 weeks, L is several weeks, XL is months. Effort assumes the MyGo widget and the current code base. Sorted by value, then effort. "Ours" uses: exists, in progress (workbench, Search, Git UI, PRs, Local History, viewers, large-file work in the brief), missing, unverified.

| # | Feature or polish item | VS Code | JetBrains | Ours | Value | Effort | Notes |
|---|---|---|---|---|---|---|---|
| 1 | Back/forward navigation history (Cmd+[ and Cmd+]) | Yes | Yes (plus Recent Locations) | missing | high | S | No code found for it. Users move between definitions and search hits all day. Keep a caret-position stack per window. |
| 2 | Keyboard shortcuts work on non-Latin layouts | Yes | Weak: IJPL-54496 has 530 votes | unverified | high | S | Match shortcuts by key code, not by character. Test with a Russian or Greek layout. |
| 3 | No telemetry, no trial banners, no ads, no AI nags | Opt-out telemetry: #176269 (1005 +1) | Trial button IJPL-222323 (225 votes); AI ads (HN 48185188) | exists by design | high | S | Cheap and visible. Say it in the first-run view. Keep it true. |
| 4 | Startup, idle CPU and memory budgets that users can see | Heavy: #148226, #165016 | Heavy: PY-81030, WEB-52943 | in progress (E2E stress budgets planned) | high | S | Add a cold-start budget to the E2E report. Show the numbers in an About view. |
| 5 | Indexing and background work shown with the reason | Basic progress | Progress bar, but no reason for re-index (HN 48185188) | missing | high | S | Status bar item: what runs, why, a Cancel button. Answers the top JetBrains complaint. |
| 6 | Inline blame | Ext: GitLens 54M installs | Annotate | exists (gitui/blame.go uses inlays) | high | S | Done. Check performance on long files. |
| 7 | Hot exit: unsaved text survives quit and crash | Yes | Yes (autosave and Local History) | missing | high | M | Session stores path, cursor and scroll only. Local History covers saves, not unsaved text. |
| 8 | Format document and format on save (external formatters) | Ext: Prettier 73M installs | Yes, built in | missing (only trim and final newline) | high | M | Call gofmt, prettier, black and so on as CLI tools. Add a format-on-save setting. |
| 9 | User and built-in snippets with tab stops | Yes; snippet extensions 22M, 18M, 12M | Live templates | missing | high | M | The MyGo widget has a snippet engine (editor/snippet.go). Add files and a picker. |
| 10 | Word-based completion from open files | Yes | Yes | missing | high | M | Widget has OnCompletion and ShowCompletions. No app code wires them. Useful for all 38 languages with no LSP. |
| 11 | Go to definition and Find usages from the symbol index | Via LSP | Yes, semantic | partial (Go to Symbol exists, no jump from caret) | high | M | Reuse internal/search/symbols. Textual and approximate, so label it so. Widget has OnDefinition. |
| 12 | Unified search (files, symbols, text, actions) | Quick Open and palette | Search Everywhere | in progress | high | M | A differentiator when it ships. Keep it fast with previews. |
| 13 | Commit window with partial staging | Basic source control | Strong | in progress | high | M | JetBrains users love the modal commit window: IJPL-177161 has 242 votes. |
| 14 | 3-way merge UI | Yes | Strong; side by side by side (HN 29887706) | in progress | high | M | Keep it non-modal if possible: IJPL-99530 has 196 votes. |
| 15 | Local History for all saves | Timeline, limited | Strong | in progress | high | M | HN item 26367410: "The only feature I miss is the local history". |
| 16 | Diff viewer: ignore whitespace, moved code | Ignore whitespace: #43026 has 635 +1 | Yes, plus IJPL-99523 smart diff (146) | in progress | high | M | Already ahead of VS Code on this. |
| 17 | Log graph | Ext: Git Graph 15.8M; #179053 has 612 | Yes | in progress | high | M | Top missing VS Code Git feature. |
| 18 | Color themes and VS Code theme import | Yes; GitHub Theme 20.7M, One Dark Pro 13M | Yes | missing (no theme setting found) | high | M | Look and feel is personal. Import VS Code theme JSON. Follow system dark and light mode. |
| 19 | LSP client: completion, hover, definitions, diagnostics, signature help | Yes, core design | Own engine | missing (widget hooks ready) | high | L | Plan says no LSP. The widget has CompletionRequest, HoverRequest, SetDiagnostics and SetInlays. Start with gopls. |
| 20 | Integrated terminal | Yes; #34442 has 844 +1 | Yes; IJPL-101423 has 204 | missing (Open in Terminal opens Terminal.app) | high | L | Plan says no terminal. HN: a terminal is the one thing that keeps users on VS Code (item 27230042). Needs a pty and a VT parser in Go. |
| 21 | Run configurations and tasks with an output panel | Tasks; Code Runner 42.9M | Yes, strong | missing | high | L | Run button in the tab bar and a Run panel. Parse file:line from output to links. |
| 22 | Test runner with gutter run icons | Testing API and ext | Strong, out of the box | missing | high | L | Needs run support first. Go test and Jest first. |
| 23 | Large files (1 GB and more) | Weak | Weak | in progress | high | L | Real differentiator. Sublime wins users with speed (HN 42862246). |
| 24 | Debugger (breakpoints, variables, stepping) | Yes, via DAP | Yes, strong | missing | high | XL | The top reason for JetBrains in HN (items 36413457, 29887706). |
| 25 | Rename symbol and other refactorings | LSP rename; limited | Strong, many | missing | high | XL | The top reason for JetBrains. Depends on a real language engine. |
| 26 | Quick fixes and intentions (Alt+Enter) | Light bulb | Yes, loved | missing | high | XL | Depends on diagnostics from LSP or inspections. |
| 27 | AI assistance (chat, inline completion, agents) | Copilot 75M installs | AI Assistant, Junie | missing | high | XL | The widget has SetSuggestion for ghost text. Out of the polish focus. |
| 28 | Extensions or plugins | Core of VS Code | Core of JetBrains | missing (plan: no plugins) | high | XL | The largest single VS Code draw. Not a near-term target. Replace the top extensions with built-in features. |
| 29 | Outline view | Yes | Structure tool window | partial (Go to Symbol and breadcrumb symbols) | med | S | Sidebar view over the existing symbol index. |
| 30 | Inline diagnostics (Error Lens style) | Ext: Error Lens 10.4M | Yes | partial (SetDiagnostics used for bracket, JSON, YAML, EOL checks) | med | S | Show message at the end of the line. Needs a setting. |
| 31 | EditorConfig | Ext: 15.5M installs | Built in | missing | med | S | Small. Sets indent, EOL, final newline per folder. |
| 32 | Scratch and untitled files | Untitled files | Scratch files | missing | med | S | Quick place for paste and notes. Local History can protect them. |
| 33 | Zen or distraction-free mode | Yes | Yes | missing | med | S | Hide sidebar, panel, status bar and tabs. One toggle. |
| 34 | Bracket pair colorization | Core since 2021; old ext 7.8M and 6.8M | Plugin | missing (bracket match exists) | med | S | Widget has BracketMatcher. Colors need a paint hook. |
| 35 | Clickable links and paths in the editor | Yes | Yes | unverified | med | S | Cmd+click on a URL or a path. |
| 36 | Auto-reveal active file in the sidebar | Yes | Yes (Autoscroll from Source) | partial (manual Reveal command) | med | S | Add a setting. |
| 37 | Compact folders in the file tree (a/b/c) | Yes | Yes (Compact Middle Packages) | missing | med | S | Small tree polish for deep projects. |
| 38 | Shortcut hints for learning | #26729 has 429 +1 | Productivity Guide, Key Promoter | partial (tooltips show keys) | med | S | Show keys in menus, palette rows and empty states. |
| 39 | Onboarding: welcome view with short walkthrough | Yes | Tip of the Day | partial (welcome.go) | med | S | Open folder, install the CLI, pick keymap. Three steps. |
| 40 | Status bar items can hide or show | #35744 has 724 +1 | Yes | exists | med | S | Done. |
| 41 | Multi-cursor and column selection | Yes; #5402 has 408 +1 | Yes | exists (widget); Alt-drag unverified | med | S | Check Alt-drag box select. |
| 42 | Code folding | Yes; folding asks: #3352 (312) | Yes | exists (widget commands, gutter, setting) | med | S | Remove from the missing list. |
| 43 | Breadcrumbs | Yes | Navigation bar | exists (with symbols) | med | S | Done. |
| 44 | Shell command (editor .) | code . is loved (HN 36413457) | idea command; IJPL-35295 (262) | exists | med | S | Done. |
| 45 | Compare two files or with clipboard | Yes | Yes | partial (Git compare only) | med | S | Reuse the diff viewer. |
| 46 | Sticky scroll | Yes | Yes | missing | med | M | Needs a widget overlay for scopes. Breadcrumb symbols exist. |
| 47 | Minimap | Yes (many turn it off) | Plugin | missing | med | M | Widget exposes the viewport for a minimap. Make it optional and cheap in idle CPU. |
| 48 | Emmet | Yes, built in | Yes | missing | med | M | HTML and CSS users. Reuse the snippet engine. |
| 49 | Auto close and auto rename HTML tag | Ext: Auto Close Tag 17.6M | Yes | missing | med | M | Needs per-language rules. |
| 50 | Spell check | Ext: Code Spell Checker 18.7M | Yes, typo inspection | missing | med | M | Use macOS NSSpellChecker if reachable. Check only comments and strings. |
| 51 | Folder-level settings (.vscode/settings.json) | Yes | .idea folder | missing | med | M | Needed for team tab size, excludes and format on save. |
| 52 | Changelists | #249974 has 98 +1 | Yes | missing | med | M | Groups edits by task. Fits the commit window. |
| 53 | Shelf and stash | Stash only; #183048 has 92 | Shelf and stash | in progress | med | M | Done soon. |
| 54 | GitHub PRs, issues, notifications | Ext: 37.6M installs | Plugin | in progress | med | M | Good fit. Keep the sign-in empty state clear. |
| 55 | Window per project, session restore, recent projects | Yes | Yes | in progress | med | M | Done soon. |
| 56 | Problems and TODO panels | Ext: Todo Tree 7.9M | Built in | in progress | med | M | Project-wide problems need a real checker: VS Code #13953 has 1511 +1. |
| 57 | File viewers (image, media, PDF, hex, Markdown, SVG) | Ext: PDF 14.7M, Hex 7.6M | Partial | in progress | med | M | Fewer extensions needed. |
| 58 | Vim mode | Ext: 9.2M; #114851 has 1371 +1 | IdeaVim plugin | missing | med | L | Loud minority. Zed and Neovim lure these users. |
| 59 | Multi-root workspaces | #43188 has 947 +1 | Modules | missing | med | L | Plan: not planned. |
| 60 | Settings profiles and sync | Yes | Plugin | missing | med | L | Low need for one machine. Defer. |
| 61 | Highlight quality (tree-sitter or semantic colors) | TextMate plus semantic; #50140 has 739 +1 | Deep per language | line lexer, 38 languages | med | L | Plan: no tree-sitter. Fine for now. |
| 62 | Remote, SSH and dev containers | Yes; dev containers 42.9M, SSH 38.9M | Gateway, weaker (HN 26367410) | missing | med | XL | Strong for VS Code. Low fit for a local macOS editor. Defer. |
| 63 | Bookmarks | Ext | Yes | missing | low | S | Small and nice. |
| 64 | Inline color swatches | Yes | Yes | missing | low | S | CSS users. |
| 65 | Keymap presets (IntelliJ, Sublime) | Ext | Built in | VS Code keymap only | low | S | Settings file already allows custom keys. |
| 66 | Detach editor into its own window | #193267 has 406 +1 | Yes | missing | low | M | Rare need. |
| 67 | Vertical tabs and tab groups | #108264 (663), #100335 (858) | Editor Groups: IJPL-59208 (196) | missing | low | M | Many votes, but tabs already match VS Code. Pinned tabs exist. |
| 68 | Macro recording | #4490 has 1229 +1 | Yes | missing | low | M | Votes are high, daily use is low. |
| 69 | HTTP client | Ext: REST Client 7.8M, Thunder 7.7M | Built in | missing | low | M | Not core. |
| 70 | Database tools | Ext: SQLTools 7.2M | DataGrip built in | missing | low | XL | Not core. |
| 71 | Profiler | Limited | Yes | missing | low | XL | Not core. |
| 72 | Notebooks | Ext: Jupyter 111M | Via DataSpell | missing | low | XL | Not core. |

## 6. Recommended shortlist (best value for effort)

**Code intelligence (inside the "no LSP" plan)**

1. Back/forward navigation (S). A top daily habit in both tools. The code has none.
2. Word completion and symbol-based go to definition and find usages (M). The widget hooks are ready and the symbol index exists. It gives all 38 languages a basic code-aware feel.
3. Snippets and built-in language snippets (M). The engine is in the widget. Snippet extensions have 22M+ installs.
4. Outline view and sticky scroll (S and M). Breadcrumb symbols already exist. Large files and long classes become easy to read.
5. Later, an optional LSP client, gopls first (L). It is the largest step toward the JetBrains feel. Needs a change to the "no LSP" decision.

**Run and terminal**

6. Integrated terminal (L). One HN user says it is the one feature that holds them on VS Code (item 27230042). Opening Terminal.app today breaks the flow.
7. Run button and tasks with a clickable output panel (L). Code Runner has 42.9M installs. It is the base for tests.

**Editing comfort**

8. Format document and format on save with external formatters (M). Prettier has 73M installs.
9. EditorConfig and folder settings (S and M). Teams need the same indent and format rules per folder.
10. Bracket pair colors (S). A tool that people installed as an extension 14M times before it went into VS Code.

**Polish and trust**

11. Hot exit (M). Unsaved text must survive quit and crash. This is what Local History users expect.
12. Background work list with reasons and Cancel, plus visible startup and idle budgets (S). It answers the top JetBrains complaint and sells our speed.
13. Color themes with VS Code theme import, and dark and light follow (M). Look is personal, and there are no themes today.
14. Inline diagnostics (Error Lens style) (S). Cheap once the problems checks exist. It makes the editor feel alive.
15. Scratch files and Zen mode (S). Small items that feel like care. Also check non-Latin keyboard shortcuts (IJPL-54496 has 530 votes).

Not recommended now: extension system, remote and dev containers, database tools, notebooks, profiler. They cost XL effort and give less feel per week of work.
