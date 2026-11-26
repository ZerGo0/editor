# JetBrains Git integration: feature inventory for a MyGo editor

Date: 2026-11-26. Scope: IntelliJ IDEA / GoLand 2026.3 (docs version seen on the fetched pages).
Language: ASD-STE100 Simplified Technical English.

## 0. How to read this report

- "Docs" links go to the official JetBrains help pages (primary source). Base: https://www.jetbrains.com/help/idea/
- "Keymap" links go to the IntelliJ Community source (primary source): https://github.com/JetBrains/intellij-community/blob/master/platform/platform-resources/src/keymaps/Mac%20OS%20X%2010.5%2B.xml and https://github.com/JetBrains/intellij-community/blob/master/platform/vcs-impl/resources/META-INF/VcsActions.xml
- The JetBrains help pages list only Windows/Linux keys. They do not list macOS keys. So the macOS column was built from the keymap XML files:
  - CONFIRMED means the macOS keymap file lists the key itself.
  - INFERRED means the action has only a "\$default" key (for example "control K") in the plugin XML. The macOS keymap then shows Cmd in place of Ctrl. Check each INFERRED key in Settings > Keymap before you copy it.
  - NONE means the action has no default key.
- Git commands come from the git manual (https://git-scm.com/docs/git-<command>). The go-git facts come from https://github.com/go-git/go-git/blob/main/COMPATIBILITY.md.
- A JetBrains feature may exist in two models. The default model uses changelists (no visible staging area). The optional model uses the Git staging area (Settings > Version Control > Git > Enable staging area). See https://www.jetbrains.com/help/idea/commit-and-push-changes.html
- Mac key symbols: Cmd = command, Ctrl = control, Opt = option.

## 1. Global rules for the git CLI layer (all features)

- Always run git as a child process. Set these for every call (See does the same, see section 16):
  - GIT_OPTIONAL_LOCKS=0 (status and diff then do not take index.lock and do not fight with the user's terminal git).
  - GIT_TERMINAL_PROMPT=0, GIT_PAGER=cat.
  - GIT_LITERAL_PATHSPECS=1 (paths with * or : are literal).
  - Pass paths after "--" and use -z with NUL parsing for every list output.
  - Use "-c core.fsmonitor=false" only if the fsmonitor daemon is a problem. Do not disable it by default for large repos (see section 17).
- Use "--no-ext-diff --no-textconv --no-color" on every diff call.
- Read machine output only from porcelain formats: "status --porcelain=v2 -z", "for-each-ref --format", "log --format", "blame --porcelain", "diff --numstat -z", "worktree list --porcelain -z", "ls-files -z".
- Use one long-lived "git cat-file --batch" process to read many blobs (base text for gutter, history, conflict sides). This avoids a process start for each file.
- Put network commands (fetch, pull, push, clone, ls-remote) on a longer timeout (See uses 5 minutes) and local commands on a short one (See uses 30 seconds).
- Use one queue for all write commands. Run read commands in parallel.

## 2. Commit tool window (changelists, staging, partial commit, amend, checks)

What it does: Shows all local changes, groups them, and creates commits.
Docs: https://www.jetbrains.com/help/idea/commit-and-push-changes.html and https://www.jetbrains.com/help/idea/managing-changelists.html

UX details:
- Tool window "Commit" on the left side. Open: Cmd+0 (CONFIRMED, ActivateCommitToolWindow = "meta 0").
- Tree of files under changelists. Default changelist is "Changes". Unversioned files go to "Unversioned Files". Conflicts go to "Merge Conflicts". Checkbox on each file, folder, changelist, and chunk.
- Toolbar: Refresh, Rollback, Group By (directory, module), Shelve silently, Show Diff, View options (Show Ignored Files, show staged and others), Expand/Collapse.
- Message field with: message history button (Ctrl+M, CONFIRMED "Vcs.ShowMessageHistory"), Reformat (wraps long lines), commit template (git config commit.template), amend checkbox with a commit picker.
- Gear / commit options (Ctrl+O, CONFIRMED "ChangesView.ShowCommitOptions"): Author, Sign-off, Commit Checks, After Commit.
- Buttons: Commit (Cmd+K, INFERRED from "control K" in VcsActions.xml), Commit and Push (Ctrl+Alt+K in the docs; macOS INFERRED Cmd+Opt+K).
- Double-click on a file opens Diff or Source (option).
- Right-click on a file: Rollback, Move to Another Changelist, Show Diff, Jump to Source, Shelve Changes, Delete, Add to .gitignore, Add to .git/info/exclude, Git > Add, Git > Show History, Annotate.

Changelists (default model):
- A changelist is a named group of changes that are not committed. It is an IDE feature, not a git feature. Git does not know about it.
- Make a changelist active: Ctrl+Space (CONFIRMED "ChangesView.SetDefault" = "control SPACE"). New changes go to the active list.
- Move to another changelist: Cmd+Shift+M (CONFIRMED "ChangesView.Move" = "meta shift M"). Same key for "Move Lines to Another Changelist" (Vcs.MoveChangedLinesToChangelist). Dialog has: list or new name, comment, Set active, Track context.
- A changelist can hold single chunks of one file. A chunk marker in the gutter has a changelist assignment menu.
- Commit each changelist on its own.
- Git CLI: none for the grouping. Store it in a file in the app data directory. For the commit, build the commit from the checked files only (see "partial commit" below).

Git staging area (optional model):
- Stage file: Add button or Ctrl+Alt+A in the docs (macOS INFERRED Cmd+Opt+A, "ChangesView.AddUnversioned" = "control alt A").
- Stage chunk: gutter marker popup > Stage. Staged changes have a hollow gutter marker.
- Stage lines: three-way viewer "Compare HEAD, Staged and Local Versions". The middle pane is the editable index.
- Git CLI: "git add -- paths", "git restore --staged -- paths" (or "git reset -q -- paths"), "git add -N" for intent to add. For hunks and lines see the next block.

Partial commit and partial stage (hunks and lines):
- In the Commit window the user ticks chunks or lines in the diff. Context menu: "Split Chunks and Include Selected Lines into Commit". In the editor: click gutter marker, type message, press "Commit this change". Unticked changes stay in the active changelist.
- Git CLI (portable method, also used by See, see section 16):
  1. Read the index text: "git show :0:path" (or "git cat-file blob :path").
  2. Compute the new index text in the app from the line selection.
  3. Write it: "git hash-object -w --stdin" then "git update-index --cacheinfo 100644,<oid>,<path>" (or "git update-index --index-info" for many files). Alternative: make a unified patch and run "git apply --cached --whitespace=nowarn -" (See does this).
  4. Unstage hunk: same with "git apply --cached -R".
  5. Commit: "git commit -m msg" (runs hooks).
- For the changelist model, build a temporary index and commit with it: "GIT_INDEX_FILE=<tmp> git read-tree HEAD", apply the selected hunks to the temp index, then "GIT_INDEX_FILE=<tmp> git commit -m msg". After the commit, refresh the real index for the committed paths ("git reset -q -- paths" or "git read-tree"). This is my design; the docs do not state how JetBrains does it.
- go-git: has no hunk-level index API. Do not use it for this.

Amend:
- Checkbox "Amend" in the message field. The chevron picks which unpushed commit to amend. Shortcut Ctrl+Alt+M (CONFIRMED "Vcs.ToggleAmendCommitMode" = "control alt M" in the macOS keymap).
- Git CLI: "git commit --amend -m msg" or "--no-edit", "--reset-author", "--author=...". For an older commit: "git commit --fixup=<sha>" then "GIT_SEQUENCE_EDITOR=true git rebase -i --autosquash <sha>^" (JetBrains "Fixup" and "Squash Into" in the Log do this; "Commit and Rebase" in the Commit button menu). See https://www.jetbrains.com/help/idea/edit-project-history.html

Commit options and checks:
- Author ("--author"), Sign-off ("-s"), skip hooks ("--no-verify"), GPG sign ("-S").
- Commit Checks (before commit): Reformat code, Rearrange code, Optimize imports, Cleanup, Update copyright, Check malicious dependencies. Advanced checks: Analyze code, Check TODO, Run configuration (can run after the commit).
- Git CLI: none. These are IDE actions. Git hooks in .git/hooks run in "git commit" unless "--no-verify". The IDE also shows a CRLF warning (setting "Warn if CRLF line separators are about to be committed") - see https://www.jetbrains.com/help/idea/resolve-conflicts.html
- For your editor (no LSP): offer pre-commit checks that you own: trim trailing whitespace, final newline, run a user command, TODO scan. Run them over the changed files before the commit.

Status source for the tree:
- "git status --porcelain=v2 -z --branch --untracked-files=all". Parse lines "1" (changed), "2" (renamed), "u" (unmerged), "?" (untracked), "!" (ignored). The "--branch" headers give oid, head, upstream, ahead/behind. Doc: https://git-scm.com/docs/git-status

## 3. Gutter change markers and rollback

What it does: Shows which lines changed against the base (index or HEAD). Colors in the default theme: light blue = modified, light green = added, grey = deleted, light orange = whitespace only.
Docs: https://www.jetbrains.com/help/idea/file-status-highlights.html (markers and the popup toolbar). Colors: Settings > Editor > Color Scheme > VCS.

UX details:
- A thin stripe in the editor gutter, next to line numbers.
- Click a marker: popup with the old text, plus toolbar: Rollback (Revert), Show Diff, copy old text, Stage (staging model), Move to changelist, Commit this change.
- Next / previous change in file: Ctrl+Opt+Shift+Down / Up (CONFIRMED "VcsShowNextChangeMarker" = "shift control alt DOWN").
- Rollback changed lines: Ctrl+Opt+Z in the Win docs is Ctrl+Alt+Z; macOS INFERRED Cmd+Opt+Z ("Vcs.RollbackChangedLines" and "ChangesView.Revert" = "control alt Z" in the default keymap).
- Rollback of files (Commit window): dialog with file list and checkbox "Delete local copies of added files". Docs: https://www.jetbrains.com/help/idea/undo-changes.html

Git CLI:
- Base text: "git show :0:path" (index) or "git show HEAD:path". See does "show HEAD:path" and uses the editor's own diff for the gutter (internal/app/git.go).
- Diff of base and buffer: do it in the app on the rope (line diff, Myers or histogram). Do not call git for each key stroke. For big files, diff only the visible window or use a time budget.
- Rollback file: "git restore --worktree -- path" (tracked), "git restore --source=HEAD --staged --worktree -- path" (also unstage), "git clean -f -- path" or delete for untracked, and "git rm --cached" for a new staged file.
- Rollback one hunk: edit the buffer in the app (replace new lines by old lines).
- Refresh trigger: file system watcher on the working tree and on .git/index, .git/HEAD, .git/refs. Then run status again.

## 4. Diff viewer

What it does: Compares two files, two versions, or two folders. Docs: https://www.jetbrains.com/help/idea/differences-viewer.html

UX details:
- Open: Cmd+D (INFERRED from "control D" in the default keymap, "Diff.ShowDiff"). Opens in an editor tab or a separate window.
- Modes: Side-by-side (two panes) and Unified (one pane). Edit allowed only in the right pane (side-by-side) or the lower line (unified).
- Colors: green = added, blue = modified, grey = deleted.
- Next / previous difference: F7 / Shift+F7 (CONFIRMED NextDiff, PreviousDiff). Next / previous file: Opt+Left / Opt+Right in the docs (Alt+Left/Right; macOS INFERRED). Jump to source: F4. Switch pane: Ctrl+Shift+Tab (CONFIRMED "Diff.FocusOppositePane"). Settings popup: Ctrl+Shift+D (CONFIRMED "Diff.ShowSettingsPopup").
- Apply left / right side: Ctrl+Cmd+Right / Ctrl+Cmd+Left (CONFIRMED "Diff.ApplyLeftSide", "Diff.ApplyRightSide").
- Chevrons in the gutter apply or append a chunk (Ctrl held = append).
- Toolbar: Collapse Unchanged Fragments, Align Changes in Side-by-Side, Synchronize Scrolling, Show Diff in Editor Tab / Separate Window.
- Ignore whitespace: None, Trim whitespaces, Ignore whitespaces, Ignore whitespaces and empty lines, Ignore imports and formatting.
- Highlight granularity: Words (default), Lines, Split changes, Characters, None.
- Three-side viewer (compare with clipboard, HEAD / staged / local).
- Gutter menu: Annotate with Git Blame.
- Multi-file diff: "Show Diff" opens a list (combined diff) of all changed files with Cmd+arrow navigation.

Git CLI:
- File list with stats: "git diff --numstat -z -M --no-ext-diff HEAD --" and "git diff --name-status -z -M HEAD --".
- Two text versions for the pane: HEAD "git show HEAD:path"; index "git show :0:path"; worktree from disk or from the buffer. Compute the diff in the app. See this way: it reads "git diff --unified=2147483647" (full context) and renders its own DiffView.
- Staged side: "git diff --cached". Branch compare: "git diff A...B" (merge base) or "git diff A B".
- Word-level: compute in the app on the changed line pairs. "git diff --word-diff=porcelain" is possible but slow for big hunks.
- Whitespace: "-w", "-b", "--ignore-blank-lines", "--ignore-space-at-eol". Algorithm: "--diff-algorithm=histogram".
- Moved code: "--color-moved" (optional).
- Your editor already has a DiffView in MyGo ui/editor. Reuse it. Feed it old text and new text. Do not parse patches except for the file list.

## 5. Git tool window and Log (graph, filters, details, actions)

What it does: Shows the commit graph with filters, details, and history actions.
Docs: https://www.jetbrains.com/help/idea/log-tab.html ; https://www.jetbrains.com/help/idea/undo-changes.html ; https://www.jetbrains.com/help/idea/edit-project-history.html ; https://www.jetbrains.com/help/idea/apply-changes-from-one-branch-to-another.html

UX details:
- Tool window "Git" (Version Control). Open: Cmd+9 (CONFIRMED "ActivateVersionControlToolWindow" = "meta 9"). Tabs: Log, Console, History (file), Update Info, Stash/Shelf.
- Four panes: Branches (left), Commits (center), Changed Files (right), Commit Details (below).
- Commits pane: graph column, subject, refs (yellow = current HEAD, green = local, violet = remote), author, date, hash. Bold = my commits. Grey = merge commits (option). Light blue background = current branch.
- Toolbar: search field (text, hash, regex, match case), Branch filter, User filter, Date filter, Paths filter, Graph options (sort by date or topologically, Show First Parent, No Merges, Collapse/Expand Linear Branches), Refresh (Ctrl+F5 in docs), Go to Hash/Branch/Tag (Ctrl+F), Cherry-pick, Highlight (My Commits, Merge Commits, Current Branch, Not Cherry-Picked), Show (Compact References, Tag Names, Long Edges, Commit Timestamp, References on the Left, Columns).
- Keys: Ctrl+L focuses search; Left / Right go to parent / child commit; F2 edits the commit message (CONFIRMED "Git.Reword.Commit" = "F2 | Shift F6"); Ctrl+Opt+N new branch (CONFIRMED "Git.CreateNewBranch" = "control alt N"). The key for "Show VCS Log" has no default; docs say to set it in Keymap.
- Commit Details: message, hash, author, email, date, GPG signature, root, branches that contain the commit (first 6, then "Show All").
- Changed Files pane: tree or flat; context menu: Show Diff, Show Diff in New Window, Compare with Local, Compare Before with Local, Edit Source, Open Repository Version, Revert Selected Changes, Cherry-Pick Selected Changes, Extract Selected Changes to Separate Commit, Drop Selected Changes, Create Patch, Get from Revision, History Up to Here, Show Changes to Parents (merge).
- Commit context menu: Copy Revision Number, Create Patch, Cherry-pick, Checkout Revision, Show Repository at Revision, Compare with Local, Reset Current Branch to Here, Revert Commit, Undo Commit, Edit Commit Message, Fixup, Squash Into, Drop Commits, Squash Commits, Interactively Rebase from Here, Push All up to Here, Add Commits to Remote Branch, Rebase <current> onto Selected Commit, New Branch, New Tag, Go to Child / Parent Commit, View in browser.
- "Enable Git Log Indexing" makes filters and file history faster and adds history search to Search Everywhere. JetBrains keeps its own index. You can use the git commit-graph file instead (see below).

Git CLI:
- Page of commits: "git log --topo-order --decorate=short --no-color --format=%x1e%H%x00%h%x00%P%x00%an%x00%ae%x00%at%x00%D%x00%s -n N --skip=S <refs>". Refs: "--branches --remotes --tags HEAD" for all, or "HEAD @{upstream}". See uses this exact shape.
- Compute graph lanes in Go from the parents list. No git graph output is needed. Lay out only the visible window plus a margin, and extend as the user scrolls.
- Filters: "--author=", "--since=", "--until=", "--grep= -i -E", "-S text", "-G regex" (pickaxe), "-- paths", "--first-parent", "--no-merges", "--simplify-by-decoration". For a hash prefix use "git rev-parse --verify <x>^{commit}".
- Which branches contain a commit: "git for-each-ref --contains <sha> refs/heads refs/remotes" and "git tag --contains <sha>".
- Details and file list: "git show -s --format=..." and "git diff-tree -r --no-commit-id --numstat -z -M <sha>" (root commit: add "--root"). Merge commit against first parent: "git diff --numstat -z -M <sha>^1 <sha>".
- Signature: "%G?" and "%GS" in the format.
- Speed for big histories: "git commit-graph write --reachable --changed-paths" and "git config fetch.writeCommitGraph true". Docs: https://git-scm.com/docs/git-commit-graph. The changed-path bloom filters speed up path-limited log. This replaces the JetBrains index.
- Cherry-pick: "git cherry-pick [-x] <sha>", multiple: "git cherry-pick a b c". No commit: "--no-commit". Merge commit: "-m 1". Continue / abort: "git cherry-pick --continue", "--abort", "--quit". State detect: .git/CHERRY_PICK_HEAD. "Not cherry-picked" highlight: "git cherry -v HEAD <branch>".
- Revert commit: "git revert [--no-edit] <sha>"; merge: "-m 1"; no commit: "--no-commit". State: .git/REVERT_HEAD.
- Reset (dialog modes Soft, Mixed, Hard, Keep): "git reset --soft|--mixed|--hard|--keep <sha>". Doc of the modes: https://www.jetbrains.com/help/idea/undo-changes.html
- Undo Commit (only own, unpushed; puts changes into a changelist): "git reset --soft HEAD~1" (for HEAD). For an older commit use the rebase path.
- Edit message of an unpushed commit: HEAD: "git commit --amend -m"; older: rebase todo with "reword".
- Drop commit: rebase todo with "drop". Squash commits: rebase todo with "squash"/"fixup". Extract changes to a separate commit: rebase todo "edit", then "git reset HEAD^", "git add" parts, two commits, "git rebase --continue".
- Interactive rebase (JetBrains: "Interactively Rebase from Here"; dialog "Rebasing Commits" with Pick, Stop to Edit, Reword, Squash, Fixup, Drop, Reset, and up/down arrows; disabled for merge commits, commits not in the current branch, protected branches): the app writes the todo list itself and runs "GIT_SEQUENCE_EDITOR=<script> GIT_EDITOR=<script> git rebase -i [--autosquash] [--rebase-merges] [--update-refs] <sha>^" (or "--root"). The script copies the todo prepared by the app over the file git gives. For reword, a second script writes the new message. Continue / abort / skip: "git rebase --continue|--abort|--skip". State: .git/rebase-merge/ and .git/rebase-apply/.
- Push All up to Here: "git push <remote> <sha>:refs/heads/<branch>".
- Create patch: "git format-patch -1 <sha> --stdout" or "git diff <sha>^ <sha>". Checkout revision: "git switch --detach <sha>". Compare with local: "git diff <sha> --".
- go-git: no rebase, no stash (COMPATIBILITY.md: "not supported"). Cherry-pick is partial. Use the git CLI.

## 6. Branches popup and the Branches pane

What it does: One popup for all branch actions. Docs: https://www.jetbrains.com/help/idea/manage-branches.html and https://www.jetbrains.com/help/idea/apply-changes-from-one-branch-to-another.html and https://www.jetbrains.com/help/idea/sync-with-a-remote-repository.html

UX details:
- Open: click the branch name in the main toolbar ("VCS widget"), or "Git | Branches". Also the VCS Operations popup: Ctrl+V on macOS (CONFIRMED "Vcs.QuickListPopupAction" = "ctrl V" in the macOS keymap; Win is Alt+`).
- Groups: Recent (up to 5), Local, Remote, Tags. Settings: Group by Directory (uses "/"), Show Recent Branches, Show Tags. Star = favorite. Type to filter. Ctrl+C copies the name. Icons on branches show incoming (blue arrow) and outgoing (green arrow) commits.
- Top actions: New Branch (Ctrl+Opt+N, CONFIRMED), Checkout Tag or Revision, New Worktree, Fetch icon, Update Project, Commit, Push.
- Per-branch menu (local): Checkout, New Branch from Selected, Checkout and Rebase onto <current>, Compare with <current>, Show Diff with Working Tree, Rebase <current> onto Selected, Merge Selected into <current>, New Worktree from, Pull into <current> Using Rebase / Merge (remote), Update, Push, Rename (option "Unset upstream branch"), Delete.
- Checkout with dirty tree: dialog "Force Checkout" or "Smart Checkout" (shelve or stash, checkout, restore; setting "Clean working tree using" Shelve or Stash). Branch checked out in another worktree: "Checkout Anyway", "Open Existing Worktree", "Cancel". Remote branch checkout makes a local tracking branch; on a name clash offers "Drop Local Commits" or "Rebase onto Remote".
- Delete shows a notification with: Restore, View Unmerged Commits, Delete Tracked Branch.
- Compare with <current>: opens a tab in the Git tool window; "Swap Branches" link. Equivalent to the log filter "branch1..branch2".
- Status bar widget shows operation state: Merge, Rebase, Cherry-pick with Abort and Continue.

Git CLI:
- List: "git for-each-ref --sort=-committerdate --format=%(refname)%00%(HEAD)%00%(upstream:short)%00%(upstream:track,nobracket)%00%(committerdate:unix)%00%(subject) refs/heads refs/remotes refs/tags".
- Checkout: "git switch <branch>"; remote: "git switch --track -c <local> <remote/branch>"; new: "git switch -c <name> [<start>]"; detached: "git switch --detach <rev>"; name check: "git check-ref-format --branch <name>".
- Force checkout: "git switch -f". Smart checkout: "git stash push -u -m msg", "git switch", "git stash pop" (handle pop conflicts).
- Rename: "git branch -m old new". Delete: "git branch -D name" (remember the sha for the Restore link: "git branch name <sha>"). Delete remote: "git push origin --delete name". Upstream: "git branch --set-upstream-to=origin/x", "git branch --unset-upstream".
- Merge: "git merge [--no-ff|--ff-only|--squash] [--no-commit] [-m msg] [--allow-unrelated-histories] <branch>". Abort: "git merge --abort". Merge in progress: .git/MERGE_HEAD.
- Rebase: "git rebase <upstream>", "git rebase --onto <new> <old> [<branch>]", with "--autosquash", "--update-refs", "--rebase-merges", "--keep-empty", "--root", "-i".
- Compare: "git log --left-right --cherry-mark A...B", "git diff --name-status -z -M A...B" (merge base) or "A B".
- Ahead / behind: "git rev-list --left-right --count HEAD...@{upstream}" (See does this) or "%(upstream:track)" in for-each-ref for all branches at once.
- Fetch: "git fetch --all --prune [--prune-tags|--tags|--no-tags]". The JetBrains setting "Fetch tags" has Auto / Sync (--prune-tags) / Always (--tags) / Never (--no-tags). Auto fetch default: every 20 minutes ("Fetch remote changes automatically", registry key git.update.incoming.info.time) - see sync page.
- Pull dialog options: "--rebase", "--ff-only", "--no-ff", "--squash", "--no-commit", "--no-verify".
- Update Project (Cmd+T, INFERRED from "control T" in VcsActions.xml). Fetches all roots and merges or rebases tracked branches. Dialog method: Merge ("pull --no-rebase"), Rebase ("pull --rebase"), Reset to remote branch ("git fetch" then "git reset --hard @{upstream}"). Then opens the "Update Info" tab. Submodules: "git submodule update" for detached ones. Implement: "git fetch --all --prune" then, per branch: "git merge --ff-only" or "git rebase" or "git pull --rebase --autostash".
- Multi-root: option "Execute branch operations on all roots".
- Pull Requests (GitHub / GitLab): out of scope for git CLI. It uses the host API (https://www.jetbrains.com/help/idea/work-with-github-pull-requests.html).

## 7. Merge conflict resolver (3-way)

What it does: Lets the user merge both sides and the base. Docs: https://www.jetbrains.com/help/idea/resolve-conflicts.html

UX details:
- When a merge, rebase, cherry-pick, stash apply, or revert stops with conflicts, a "Conflicts" dialog opens. Per file: Accept Yours, Accept Theirs, Merge (opens the 3-way tool). Button "Resolve All Simple Conflicts". Later access: "Merge Conflicts" node in the Commit window; Resolve and Abort in the status bar.
- 3-way tool: left = local (yours, read only), right = incoming (theirs, read only), center = result (editable, starts as the base). Per change: Accept (arrow) or Ignore (x) per side. Toolbar: Apply All Non-Conflicting Changes, Apply Non-Conflicting Changes from Left / Right, Resolve simple conflicts (same-line start/end edits), "Compare contents" (Base vs Middle). Right-click on a conflict: Resolve using Left / Right. Button "Apply Changes" and "Save and Close". "Accept and Finish" when all conflicts are gone. Undo: Ctrl+Z (merge ops are undoable).
- Setting: Tools | Diff & Merge > "Automatically apply non-conflicting changes".
- Context-menu in the Conflicts dialog: "Revert conflict resolution" (restore the conflict state).

Git CLI:
- Find conflicts: "git status --porcelain=v2 -z" lines "u" with the XY code (UU, AA, DD, AU, UA, DU, UD). Or "git ls-files -u -z".
- Get the three sides: stage 1 = base, stage 2 = ours, stage 3 = theirs: "git show :1:path", "git show :2:path", "git show :3:path" (or "git cat-file blob <oid>" using the oids from "ls-files -u").
- Auto-merge non-conflicting hunks: "git merge-file -p --diff3 ours base theirs" on temp files (exit code = number of conflicts). Or do a diff3 in the app. This gives you the "Apply all non-conflicting" step.
- Accept one side: "git checkout --ours -- path" or "git checkout --theirs -- path". In a rebase the meaning is swapped: "ours" is the branch you rebase onto. Show the right labels in the UI.
- Re-create markers: "git checkout -m -- path" or "git checkout --conflict=diff3 -- path". Config: "merge.conflictStyle zdiff3".
- Mark resolved: write the result file, then "git add -- path". Deleted-side conflicts: "git rm -- path" or "git add -- path".
- Continue: "git merge --continue", "git rebase --continue", "git cherry-pick --continue", "git revert --continue". Abort: same with "--abort".
- Binary files: only Accept Yours / Theirs.
- "git rerere" can remember resolutions (optional).
- See (section 16) reads the markers in the working file and shows two panes (current and incoming) with Keep current / incoming / both. It does not show the base pane. A JetBrains-like tool needs the three stage blobs.

## 8. Annotate (blame)

What it does: Shows author and date per line in the gutter. Docs: https://www.jetbrains.com/help/idea/investigate-changes.html

UX details:
- Right-click the gutter > "Annotate with Git Blame" and "Close Annotations". No default key (docs: set under Keymap > Version Control Systems > Git > Annotate). NONE in the keymap.
- Column shows author, date (format set in Settings > Date Formats), and optionally the revision. Lines changed in the current revision are bold with an asterisk. Colors per commit. Hover: popup with commit and a link that opens the Log at that commit. Right-click: Annotate Revision, Annotate Previous Revision, Hide Revision, Restore Hidden Revisions, Open on GitHub.
- Options: Ignore Whitespaces ("-w"), Detect Movements Within File ("-M"), Detect Movements Across Files ("-C"), Show Commit Timestamp.
- Code author inlay hint above each function (Code Vision): Settings > Editor > Inlay Hints > Code vision > Code author.
- Works in the diff viewer gutter too.

Git CLI:
- "git blame --porcelain [-w] [-M] [-C] [--ignore-revs-file=F] --contents - -- path" with the buffer text on stdin (See does this, so unsaved edits are blamed correctly). Range: "-L start,end". Stream: "--incremental" (lines come as soon as git finds them; good for large files).
- Annotate Previous Revision: porcelain output has "previous <sha> <path>" for each commit. Blame again at that sha: "git blame --porcelain <sha>^ -- <old path>" (or "git blame <sha> -- path").
- Large files: blame is O(history * size). Cap by file size and show a notice. Use "-L" for the visible range only.
- go-git: has blame (COMPATIBILITY.md: supported), but it is slower and has no -w/-M/-C and no ignore-revs. Prefer the git CLI.

## 9. File history and selection history

Docs: https://www.jetbrains.com/help/idea/investigate-changes.html

UX details:
- File history: "Git | Selected File | Show History" or the Git menu on a file. Opens tab "History" in the Git tool window. Action "Vcs.ShowTabbedFileHistory" has no default key (NONE; VcsActions.xml line 4: https://github.com/JetBrains/intellij-community/blob/master/platform/vcs-impl/resources/META-INF/VcsActions.xml).
- Toolbar: Branch filter, Refresh, Show Diff (Ctrl+D), Show All Affected Files (Ctrl+Cmd+A, CONFIRMED "VcsHistory.ShowAllAffected" = "meta ctrl A"), View options (commit timestamp, details, diff preview), Open on GitHub. Rename history column.
- Selection history: select text, then "Git | Current File | Show History for Selection" ("Vcs.ShowHistoryForBlock", no default key). With no selection it shows the history of the current line.

Git CLI:
- File: "git log --follow --format=<fmt> --name-status -z -- path" (See does this in internal/git/revision.go and history.go). "--follow" works for one file only. Add "--full-history" so parents stay linked in the graph.
- Show the file at a commit: "git show <sha>:path". Diff of one file in a commit: "git diff <sha>^ <sha> -- path".
- Selection: "git log -L <start>,<end>:<path> --format=<fmt> -s"? The "-L" option prints patches per commit. Parse the patch to show the changed range, or run without "-s". Doc: https://git-scm.com/docs/git-log (option "-L"). Function form: "-L :funcname:path" needs a userdiff driver; with your own lexers, use line ranges.
- Cost: "-L" and "--follow" are slow on large histories. Run in the background, stream results.

## 10. Shelve vs stash

Docs: https://www.jetbrains.com/help/idea/shelving-and-unshelving-changes.html (page also titled "Shelve and unshelve changes"; fetched content from this URL) and the "Shelf vs. Stash" section of the shelve page linked from it.

Differences (from the docs):
| Item | Shelf | Stash |
|---|---|---|
| Creator | IDE patch files | git |
| Where applied | Only in the IDE | In the IDE or outside |
| Scope | Chosen files or a changelist | All uncommitted changes |
| Untracked files | Not possible | Possible ("Include untracked") |
| Combined view | Setting "Combine stashes and shelves in one tab" | same |

UX details:
- Shelf: "Shelve Changes" dialog (name = message). "Shelve Silently" Cmd+Shift+H (INFERRED, "ChangesView.ShelveSilently" = "shift control H" in default keymap). "Save to Shelf" copies without reverting. Unshelve: Cmd+Shift+U (INFERRED "ShelveChanges.UnshelveWithDialog" = "control shift U"); Unshelve Silently Ctrl+Alt+U in the docs ("ChangesView.UnshelveSilently" is listed in VcsActions.xml; its key was not checked in the XML, so treat Cmd+Opt+U as unverified). Dialog fields: changelist, comment, Set active, Track context, "Remove successfully applied files from the shelf". Drag from the Shelf tab to the Commit tab to unshelve silently.
- Stash: "Git | Stash Changes" (message, "Keep index", "Include untracked" options as git flags), "Git | Unstash Changes" (choose stash, branch, "Pop stash", "Reinstate index"). A "Stash" tab lists stashes with a diff.
- Stash cannot apply to a working copy with conflicting changes.

Git CLI:
- Stash: "git stash push [-u|--include-untracked] [-k|--keep-index] [-m msg] [-- paths]". List: "git stash list -z --date=raw --format=%H%x1f%ct%x1f%gd%x1f%gs" (See). Show files: "git stash show --include-untracked --numstat -z <ref>" or "git diff <ref>^1 <ref>". Untracked files live in the third parent "<ref>^3" (See reads this). Apply: "git stash apply --index <sha>" (fallback without "--index"). Pop: "git stash pop". New branch: "git stash branch <name> <ref>". Drop: "git stash drop <ref>"; clear: "git stash clear". Use the commit hash, not "stash@{n}", for stability (See does this).
- Shelf: your own. Save a binary patch "git diff --binary HEAD -- paths" plus a name and a time in the app data folder. Apply: "git apply [--3way] patch". Unversioned files cannot be shelved in JetBrains, but you can add them via "git add -N" first.
- Simpler plan: ship stash first. Ship the shelf later or skip it.

## 11. Push dialog

Docs: https://www.jetbrains.com/help/idea/commit-and-push-changes.html

UX details:
- Open: Cmd+Shift+K (INFERRED from the docs' Ctrl+Shift+K), or "Git | Push", or "Push All up to Here" in the Log.
- Left tree: repositories and outgoing commits per branch. Right: changed files of the selected commit (or all). Click the remote name to change or define a remote ("Define remote" when none). Click the target branch name to type a new one; "Edit all targets".
- "Push tags" checkbox (off by default): All ("--tags") or Current Branch ("--follow-tags").
- Button Push, drop-down Force push (always "--force-with-lease"). Force push is off for protected branches (setting "Protected branches").
- Commit marked with asterisk when the author differs. Ctrl+Q shows commit info.
- "Push Rejected" dialog: Merge or Rebase then push again; option "Remember and silently update in future".
- Commit and Push: Ctrl+Alt+K in docs (Cmd+Opt+K INFERRED).

Git CLI:
- Outgoing commits: "git log @{push}..HEAD" or "git rev-list @{upstream}..HEAD" (See). For a new branch: "git log HEAD --not --remotes=origin".
- Push: "git push [--set-upstream] <remote> <local>:refs/heads/<remote branch>", "--force-with-lease[=ref:expected]", "--tags", "--follow-tags", "--dry-run", "--no-verify", "-o option". Delete remote branch: "--delete". Progress: "--progress" (stderr).
- Rejection detection: parse stderr for "rejected" / "non-fast-forward" / "fetch first".
- Credentials: Do not ask in the app at first. Let git use its credential helper and SSH agent. See sets GIT_TERMINAL_PROMPT=0 so git never blocks. For HTTPS prompts you need an askpass helper ("GIT_ASKPASS" pointing to your own small program or the app itself).
- Pre-push hooks run in "git push".

## 12. Tags

Docs: https://www.jetbrains.com/help/idea/use-tags-to-mark-specific-commits.html

UX: "Git | New Tag" (dialog: Git Root, Tag name, Commit (hash or expression like branch~3), Message, Force). Empty message = lightweight tag; non-empty = annotated. Log context menu "New Tag". Tags node in the Branches popup and pane: Checkout, Compare, Merge, Push to origin, Delete. Delete from the tagged commit menu "Tag X | Delete" (and remote delete option). Settings: "Fetch tags" (Auto, Sync, Always, Never).
Git CLI:
- Create: "git tag [-a -m msg | --no-sign] [-f] <name> <sha>". Delete: "git tag -d <name>"; remote: "git push origin --delete <name>" (or ":refs/tags/<name>"). Push one: "git push origin refs/tags/<name>". Fetch policy: see section 6. List: "git for-each-ref --format=... refs/tags" with "%(objecttype) %(*objectname) %(taggerdate) %(contents:subject)". Checkout: "git switch --detach <tag>".
- go-git supports tags, but use the CLI for consistency.

## 13. Remotes

Docs: https://www.jetbrains.com/help/idea/set-up-a-git-repository.html
UX: "Git | Manage Remotes" dialog (Add with Alt+Insert, Edit, Remove). Remotes also show under the Branches pane (right-click: Edit Remote, Remove Remote). "Define remote" in the Push dialog. Clone dialog: URL, directory, "Shallow clone with a history truncated to N". "Git | Unshallow Repository".
Git CLI: "git remote -v", "git remote add <n> <url>", "git remote set-url <n> <url>", "git remote rename <old> <new>", "git remote remove <n>", "git remote get-url <n>" (See, for web links), "git ls-remote --heads <url>" (check access), "git clone --depth N <url> <dir>", "git fetch --unshallow".

## 14. Worktrees

Docs: https://www.jetbrains.com/help/idea/use-git-worktrees.html
UX: Tab "Worktrees" in the Git tool window (Cmd+9 then the chevron menu) and "Git | Worktrees". Toolbar: New Worktree (+), Delete, Prune. Context menu: Open Worktree, Lock / Unlock. Main worktree in bold; locked icon; prunable shown dim. Dialog "New Worktree": From branch (local, remote, tag), "New branch" checkbox, Project name, Location, "Copy local project setup" (.worktreeinclude file and a setup script; a script times out after 5 minutes). Branch checked out elsewhere: dialog "Worktree Conflict" (Open Existing Worktree / Checkout Anyway).
Git CLI: "git worktree list --porcelain -z", "git worktree add [-b <new>] <path> [<commit-ish>]", "git worktree remove [-f] <path>", "git worktree prune", "git worktree lock|unlock [--reason r] <path>", "git worktree move". Each worktree is a separate root with its own index and HEAD. go-git: worktree support is partial ("Not all flags nor subcommands are supported").

## 15. Submodules, ignored files, local history

Submodules:
- Docs have little text. The clone page says submodules are cloned and added as project roots (https://www.jetbrains.com/help/idea/set-up-a-git-repository.html). Update Project runs "git submodule update" for detached submodules (https://www.jetbrains.com/help/idea/sync-with-a-remote-repository.html). Each submodule is a separate Git root with its own branch and commit (blog: https://blog.jetbrains.com/idea/2018/09/intellij-idea-2018-3-eap-git-submodules-jvm-profiler-macos-and-linux-and-more/). The docs are not a full specification; known gaps are on YouTrack (https://youtrack.jetbrains.com/issue/IJPL-115218/Improve-Git-Submodule-support).
- Git CLI: "git submodule status --recursive", "git submodule update --init --recursive", "git submodule add <url> <path>", "git submodule foreach", "git config -f .gitmodules --list". Status v2 has a submodule field "S<c><m><u>" (commit changed, modified, untracked). Use "git -C <sub> ..." for the commands inside. go-git: submodule is supported but "deinit" is not.

Ignored files:
- Docs: https://www.jetbrains.com/help/idea/ignoring-files.html
- UX: Right-click a file > "Git | Add to .gitignore" or "Add to .git/info/exclude". Ignored files are grey in the tree. Commit window view option "Show Ignored Files". Adding an ignored file asks for a force add. Only unversioned files can be ignored.
- Git CLI: "git ls-files --others --ignored --exclude-standard --directory -z" (See), "git status --porcelain=v2 -z --ignored=matching", "git check-ignore -v -z --stdin" (to show which rule matches; go-git: "check-ignore" is not supported), "git add -f -- path". Write the pattern lines into .gitignore or .git/info/exclude yourself.
- Big trees: use "--directory" so git does not list every file in ignored folders (node_modules).

Local history (not git):
- Docs: https://www.jetbrains.com/help/idea/local-history.html
- What it does: records the state of the project during editing, test runs, deploys, and commits, also for changes made outside the IDE. Works without VCS. Scope: file, selection, folder, project.
- UX: right-click in the editor or Project view > "Local History | Show History". Menu "File | Local History | Put Label". No default key. Revision list with a diff on the right. Actions: Revert Selected and Later Changes, chevron to copy a fragment into the file, Revert Selection for a deleted file, Show Difference, Create Patch, search the content of revisions. System labels are added automatically. Retention: 5 working days by default (Advanced Settings; JVM key -DlocalHistory.daysToKeep=<days>, 0 disables it).
- Implementation idea (my design): on save and before each external change, store a snapshot (content-hash addressed, zstd compressed) in an app data directory. Keep a per-file journal (time, hash, label). Keep a size cap. Skip files over a size limit (for example 10 MB). A persistent rope buffer lets you take cheap in-memory snapshots, but you must still write them to disk. For 1 GB files store no content (store only a note).

## 16. What See's internal/git does (ideas only; license: none, do not copy code)

Repo: https://github.com/wibus-wee/see (cloned shallow into <scratch>/clones/see). The repository has no license. This section lists behavior only. Files: internal/git/*.go and internal/app/git*.go.

Runner (internal/git/git.go, remote.go):
- Every call: "git -c core.fsmonitor=false -C <dir> <args>", env GIT_OPTIONAL_LOCKS=0, GIT_TERMINAL_PROMPT=0, GIT_PAGER=cat, GIT_LITERAL_PATHSPECS=1. Output is combined stdout and stderr. 30 second timeout. A separate "Network" runner with 5 minute timeout and without GIT_OPTIONAL_LOCKS. It detects "offline" from stderr text ("could not resolve host", "connection refused", ...) and stops background fetches.
- Background auto fetch: "git fetch --all --prune", with back-off after failures.
- Source: https://github.com/wibus-wee/see/blob/main/internal/git/git.go and https://github.com/wibus-wee/see/blob/main/internal/git/remote.go and https://github.com/wibus-wee/see/blob/main/internal/app/remote.go

Feature map:
| JetBrains feature | See | Commands in See |
|---|---|---|
| Status / Commit window tree | Yes (Git navigator: Staged, Unstaged, Conflicted) | "status --porcelain=v2 --branch -z --untracked-files=all"; "rev-parse --show-toplevel", "--short HEAD", "--verify --quiet MERGE_HEAD" |
| Stage / unstage | Yes (files, selection) | "add -- paths", "reset -q -- paths" |
| Commit | Yes (message + commit) | "commit -m msg". No amend, no sign-off, no checks |
| Changelists | No | - |
| Partial staging (hunks / lines) | Yes | internal/git/index_patch.go: reads "diff --full-index --unified=2147483647 [--cached]"; builds a patch and applies it with "-c apply.ignoreWhitespace=false apply --cached --whitespace=nowarn -". For untracked files it uses a temp GIT_INDEX_FILE and "add --intent-to-add" to get a patch |
| Gutter markers and peek | Yes (diff of base vs buffer, popup "peek") | "ls-files --error-unmatch -- path", "show HEAD:path", "rev-parse --git-path index" (change detection) |
| Rollback | Yes, file level | "restore --worktree -- path" |
| Diff viewer | Yes (MyGo editor DiffView; staged/unstaged/commit/branch) | "diff ... --unified=2147483647"; "diff --no-index" for untracked and for CLI compare |
| Blame | Yes, uses buffer text | "blame --porcelain --contents - -- rel" (stdin = buffer) |
| Log + graph | Yes (graph lanes computed in Go, paging, filter text, jump to date, Load More) | "log --topo-order --decorate=short --no-color --format=%x1e%H%x00%h%x00%P%x00%an%x00%ae%x00%at%x00%D%x00%s%x00%(trailers:key=Co-authored-by,...) --skip=N -n N" plus "HEAD @{upstream}" or "--branches --remotes --tags HEAD"; "rev-list --count"; "rev-list @{upstream}..HEAD" and "HEAD..@{upstream}" for arrows; "log -1 --before=DATE 23:59:59 --format=%H" |
| Commit details and files | Yes | "show -s --format=%cn%x00%ce%x00%ct%x00%B"; "diff --numstat -z -M <parent> <hash>"; "diff-tree --root -r --no-commit-id --numstat -z"; "diff --no-ext-diff --no-color -M <parent> <hash>" |
| File history | Yes | "log --follow --full-history ... -- path", "log --follow --format=%x00%H --name-status -z HEAD -- path", "show <hash>:path" |
| Selection history | No | - |
| Cherry-pick | Yes (log menu; no continue/abort UI seen) | "cherry-pick <hash>" |
| Revert, reset, interactive rebase, drop, squash, fixup | No | - |
| Branches list | Yes (picker: current, local, remote) | "for-each-ref --sort=-committerdate --format=%(refname)%00%(HEAD)%00%(upstream:short)%00%(upstream:track,nobracket)%00%(committerdate:unix)%00%(subject) refs/heads refs/remotes", "for-each-ref ... refs/tags" |
| Checkout / new branch | Yes | "switch <name>", "switch --track -c <local> <remote>", "switch -c <name>", "check-ref-format --branch" |
| Compare branch / preview | Yes | "merge-base", "diff --numstat -z -M <mergebase>", "diff --name-status -z --find-renames HEAD <target>", "log -n N <target>", "ls-files --others --exclude-standard -z" |
| Merge / rebase | No | - |
| Fetch / pull / push | Yes (status bar; pull ff-only) | "fetch --all --prune", "pull --ff-only", "push", "push --set-upstream <remote> HEAD" |
| Tracking info | Yes | "rev-parse --abbrev-ref --symbolic-full-name @{upstream}", "rev-list --left-right --count HEAD...@{upstream}" |
| Push dialog | No (one click push) | - |
| Merge conflict resolver | Partial: 2 panes (Current, Incoming) with Keep current / incoming / both per hunk, Save and Stage. No base pane. No whitespace-only or auto merge | "ls-files -u -z -- path", "check-attr -z merge conflict-marker-size -- path", "cat-file -s <oid>", "cat-file blob <oid>" (used to ignore example markers), then parse the markers in the worktree file, then "add -- path" |
| Stash | Yes (save, apply, drop, list; untracked via ^3) | "stash push --include-untracked [-m]" (with GIT_LITERAL_PATHSPECS=0), "stash list -z --date=raw --format=%H%x1f%ct%x1f%gd%x1f%gs", "stash apply --index <hash>" with fallback, "stash drop <ref>", "rev-parse --verify --quiet <hash>^3", "log -1 --format=%P%x00%s" |
| Shelf | No | - |
| Tags | Create only (annotated optional) | "tag --no-sign [--annotate --message m] -- name hash". No delete / push tag |
| Remotes | Read only | "remote", "remote -v", "remote get-url <n>" (builds GitHub / GitLab web links, "compare" links) |
| Worktrees | No | - |
| Submodules | No | - |
| Ignored files | Yes (dim in tree) | "ls-files --others --ignored --exclude-standard --directory -z" |
| Local history | No | - |

Ideas worth taking from See (behavior only):
- Pass the editor buffer to "git blame --contents -" so unsaved edits are correct.
- Compute graph lanes in the app; page the log with "--skip/-n".
- Use "rev-list @{upstream}..HEAD" for the incoming / outgoing arrows.
- Use a fingerprint of "for-each-ref" output and the index path to know when the log is stale.
- Apply index patches with "git apply --cached" and a temp GIT_INDEX_FILE for untracked files.
- Stop background network calls when the machine is offline; use back-off.
- Use "--intent-to-add" in a temp index to diff untracked files like tracked ones.

## 17. Notes on 1 GB files, low memory, go-git

- Do not run "git diff", "git blame" or "git log -L" on huge files without a size gate. Show a notice and allow the user to continue. Git itself treats files above core.bigFileThreshold (default 512 MiB, git docs: https://git-scm.com/docs/git-config) as binary for delta and diff purposes. Gutter markers for a 1 GB file: diff only against HEAD with a streaming line diff and a time budget, or turn markers off over a size limit.
- Large repos: enable "core.untrackedCache=true", "core.fsmonitor=true" (built-in daemon on macOS) and "feature.manyFiles=true" in the repo. Doc: https://git-scm.com/docs/git-status (untracked cache and fsmonitor) and https://git-scm.com/docs/git-update-index. See turns fsmonitor off per call; for a big repo this makes status slow, so measure before you copy this.
- Run "git status" with "--untracked-files=normal" for the first paint; use "all" only for the Commit tree. Use "--no-renames" for speed when the repo is huge.
- go-git (https://github.com/go-git/go-git/blob/main/COMPATIBILITY.md): stash = not supported; rebase = not supported; merge = fast-forward only; cherry-pick = partial (default ort strategy, only "theirs" and "ours" options); worktree = partial; submodule = supported but "deinit" missing; check-ignore = not supported; blame, sparse-checkout, status = supported. Conclusion: use the git CLI for all write operations. If you want an in-process helper, use go-git only to read objects (for example, to read a blob or walk commits without a process start); measure it against "git cat-file --batch" first.
- Memory: parse streams line by line with bufio, not as one string. Use "-z" and a NUL splitter. Limit the log window to a few thousand commits in memory.

## 18. Search Everywhere hooks (Git related)

- Search Everywhere: double Shift (macOS; there is no key in the XML, the action is bound to a double-key gesture). Tabs: All, Classes, Files, Symbols, Actions, Git (with Log indexing: commits by hash and message). Tab key switches tabs (CONFIRMED "SearchEverywhere.NextTab" = TAB). Cmd+Shift+A opens "Find Action" (INFERRED).
- Git content for a Search Everywhere: commits (message, hash, author), branches, tags, stash, git actions (command palette). With "git log --format=... -i --grep=<q>" and "git for-each-ref" you can fill this. A commit-graph and an own small index make it fast. See https://www.jetbrains.com/help/idea/log-tab.html ("Enable Git Log Indexing ... enables searching history in Search Everywhere").

## 19. Summary of macOS shortcuts

| Action | macOS key | Status | Source |
|---|---|---|---|
| Commit tool window | Cmd+0 | CONFIRMED | Mac keymap, "ActivateCommitToolWindow" |
| Git tool window | Cmd+9 | CONFIRMED | Mac keymap, "ActivateVersionControlToolWindow" |
| VCS Operations popup | Ctrl+V | CONFIRMED | Mac keymap, "Vcs.QuickListPopupAction" |
| Move to changelist | Cmd+Shift+M | CONFIRMED | Mac keymap, "ChangesView.Move" |
| Set active changelist | Ctrl+Space | CONFIRMED | Mac keymap |
| Amend toggle | Ctrl+Opt+M | CONFIRMED | Mac keymap |
| Commit options | Ctrl+O | CONFIRMED | Mac keymap |
| Message history | Ctrl+M | CONFIRMED | Mac keymap |
| Next / prev change in file | Ctrl+Opt+Shift+Down / Up | CONFIRMED | Mac keymap |
| Next / prev diff | F7 / Shift+F7 | CONFIRMED | Mac keymap |
| Apply left / right side | Ctrl+Cmd+Right / Left | CONFIRMED | Mac keymap |
| Switch diff pane | Ctrl+Shift+Tab | CONFIRMED | Mac keymap |
| Diff settings popup | Ctrl+Shift+D | CONFIRMED | Mac keymap |
| Show all affected files | Ctrl+Cmd+A | CONFIRMED | Mac keymap |
| New branch | Ctrl+Opt+N | CONFIRMED | Mac keymap |
| Edit commit message | F2 | CONFIRMED | Mac keymap, "Git.Reword.Commit" |
| Commit | Cmd+K | INFERRED | VcsActions.xml, "control K" |
| Update Project | Cmd+T | INFERRED | VcsActions.xml, "control T" |
| Push | Cmd+Shift+K | INFERRED | docs: Ctrl+Shift+K |
| Commit and Push | Cmd+Opt+K | INFERRED | docs: Ctrl+Alt+K |
| Rollback | Cmd+Opt+Z | INFERRED | default keymap, "control alt Z" |
| Add (stage) | Cmd+Opt+A | INFERRED | VcsActions.xml, "control alt A" |
| Show diff | Cmd+D | INFERRED | default keymap, "control D" |
| Shelve silently | Cmd+Shift+H | INFERRED | VcsActions.xml, "shift control H" |
| Unshelve | Cmd+Shift+U | INFERRED | default keymap |
| Find Action | Cmd+Shift+A | INFERRED | default keymap |
| Annotate, file history, selection history, Local History, Log | none | NONE | docs and XML |

Note: The Win/Linux key list is at https://www.jetbrains.com/help/idea/main-version-control-shortcuts.html (Alt+`, Ctrl+K, Ctrl+T, Ctrl+Alt+Z, Ctrl+Shift+K, Alt+9, Alt+0).
Caveat: the keymap files give "control" for the Win default and the macOS keymap swaps Ctrl for Cmd. I did not run the IDE to confirm this swap. Confirm each INFERRED key in the IDE before you copy it. Keys that a user has changed are not shown.

## 20. Suggested build order (value first)

Phase 1: Read-only core (highest value, low risk)
- Repo detect, root, "status --porcelain=v2 -z --branch", file watcher, tree view with status colors and ignored dim.
- Gutter markers (in-app diff of the rope against HEAD / index text) and click popup with old text.
- Diff viewer (reuse DiffView): file vs HEAD, staged vs HEAD, side-by-side and unified, word-level, whitespace options.
- Branch name and ahead / behind in the status bar.
- Reason: used all day; no write risk; fits your editor widget.

Phase 2: Commit workflow
- Commit tool window: stage / unstage, rollback (file and hunk), commit with amend, sign-off, author. Partial staging of hunks and lines with "apply --cached" or "update-index --cacheinfo". Keep the staging model first. Add changelists after, as an app-side layer.
- Hooks via "git commit". Pre-commit checks of your own (whitespace, final newline, TODO).
- Reason: this is the main JetBrains value; See already shows the method.

Phase 3: History
- Log tool window: paged "git log", lane graph in Go, filters (branch, author, date, path, text), commit details, changed files with diff, Go to hash / branch / tag, file history ("--follow"), selection history ("-L"), annotate ("blame --porcelain --contents -", then "--incremental" for big files). Add "commit-graph write --changed-paths" in the background.
- Commit actions: cherry-pick, revert, reset (4 modes), new branch, new tag, checkout revision, copy hash, create patch.
- Reason: second most used. Big files need the size gates from section 17.

Phase 4: Branches and sync
- Branches popup (checkout, new, rename, delete, compare, merge, rebase), fetch, pull (merge / rebase / ff-only), push dialog (outgoing list, force-with-lease, tags, protected branches), Update Project, stash and smart checkout, remotes dialog, tags push and delete. Credential helper and askpass.
- Operation state machine for merge / rebase / cherry-pick / revert with Continue and Abort (read .git/MERGE_HEAD, CHERRY_PICK_HEAD, REVERT_HEAD, rebase-merge/).
- Reason: needed to leave the terminal.

Phase 5: Conflicts and history editing
- 3-way merge tool (stages 1, 2, 3; "git merge-file" for the auto merge; result pane editable). Interactive rebase UI from the log (own todo editor with GIT_SEQUENCE_EDITOR), reword, squash, fixup, drop, extract changes. Undo Commit.
- Reason: hard UI and many edge cases; do it after the base is stable.

Phase 6: Long tail
- Worktrees tab, submodules (status, update, roots), shelf (app patches), Local History (snapshot store with labels and retention), "Git" tab in Search Everywhere with commit search, GitHub / GitLab pull requests via the host API.
- Reason: lower usage, or not git at all (Local History).

## 21. Open points and risks

- macOS shortcuts marked INFERRED need a check in the IDE (see section 19).
- The JetBrains help pages did not give the internal method for partial commit in the changelist model. The temp-index method in section 2 is my design.
- The submodule section is thin. JetBrains docs have no page for it. The blog and YouTrack link above show that behavior is limited.
- Some doc pages gave a summary only (the fetch tool shortens pages). Lists such as the Log context menu come from those summaries and may miss items in newer builds.
- See has no license. Use it only to learn what to build. Write your own code.
