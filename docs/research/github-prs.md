# GitHub Pull Requests features for a MyGo editor, using the gh CLI

Date: 2026-11-26. Method: read the source of microsoft/vscode-pull-request-github (shallow clone, main branch), the local gh 2.105.0 help text, and one live gh call. Claims have a source link. Items marked UNVERIFIED were not tested.

Base URL for extension sources: https://github.com/microsoft/vscode-pull-request-github/blob/main/
Short names: EXT = vscode-pull-request-github. GQL = GraphQL.

## 0. Key decisions (summary)

1. EXT does almost all work with GraphQL plus a few REST calls. It uses the VS Code token, not gh. Source: [src/github/queries.gql](https://github.com/microsoft/vscode-pull-request-github/blob/main/src/github/queries.gql), [queriesShared.gql](https://github.com/microsoft/vscode-pull-request-github/blob/main/src/github/queriesShared.gql), [credentials.ts](https://github.com/microsoft/vscode-pull-request-github/blob/main/src/github/credentials.ts).
2. The gh porcelain (`gh pr ...`) covers about 60 percent of the features. It has no inline review comments, no thread resolve, no pending review, and no suggestions. For these, use `gh api graphql`.
3. Best design: use gh only to find the host and the token. Then call the REST and GraphQL APIs with your own Go `net/http` client. This gives ETags, low CPU, and no process spawn cost. Keep `gh` subprocess calls for the simple actions (checkout, create, merge, `gh issue develop`).
4. Do not copy EXT code for UI. EXT is MIT, so reading and reuse of queries is allowed with the licence notice. Check the licence file before you copy any text: [LICENSE](https://github.com/microsoft/vscode-pull-request-github/blob/main/LICENSE).

## 1. Feature inventory of EXT, and the gh way

Count: the package.json has about 230 commands and about 75 settings. Source: [package.json](https://github.com/microsoft/vscode-pull-request-github/blob/main/package.json), [package.nls.json](https://github.com/microsoft/vscode-pull-request-github/blob/main/package.nls.json). Views: PR list "pr:github", issues "issues:github", notifications "notifications:github", create PR webview, compare changes files and commits, "prStatus:github" (changed files in review mode), active PR webview, conflict resolution.

### 1.1 PR list views and queries

EXT features:
- Tree with categories. Default queries: "Copilot on my behalf", "Local Pull Request Branches", "Waiting for my review" (`review-requested:${user}`), "Created by me" (`author:${user}`), "All open". Setting `githubPullRequests.queries` is a list of {label, query}. Variables: ${owner}, ${repository}, ${user}. Source: package.json (setting default value) and [README](https://github.com/microsoft/vscode-pull-request-github/blob/main/README.md).
- Query uses GitHub search syntax. EXT calls REST `search.issuesAndPullRequests` with `advanced_search` ([folderRepositoryManager.ts L1476](https://github.com/microsoft/vscode-pull-request-github/blob/main/src/github/folderRepositoryManager.ts)). Pagination with load more.
- Settings: `autoRefreshQueries` (seconds, 0 = off), `remotes` (default origin, upstream), `showPullRequestNumberInTree`, `pullRequestAvatarDisplay`.
- "Local Pull Request Branches" finds PRs for local branches. It uses the GQL query `PullRequestForHead(owner,name,headRefName)`.
- Edit query, refresh, open on github.com, copy link.

gh way:
- List: `gh pr list -R HOST/OWNER/REPO --search "<query>" --state open --json number,title,author,isDraft,updatedAt,headRefName,baseRefName,reviewDecision,statusCheckRollup,labels,url --limit 100`.
- Cross-repo "waiting for my review": `gh search prs --review-requested=@me --state=open --json ...`. The search JSON has fewer fields than `gh pr list`.
- Better for speed: one GQL `search(type: ISSUE, first: 50, query: $q) { issueCount pageInfo{hasNextPage endCursor} nodes { ... on PullRequest { number title isDraft updatedAt reviewDecision author{login avatarUrl} headRefName statusCheckRollup: commits(last:1){nodes{commit{statusCheckRollup{state}}}} } } }`. This is the `Issues($query)` query in EXT ([queries.gql L291](https://github.com/microsoft/vscode-pull-request-github/blob/main/src/github/queries.gql)). One request gives list plus status icons.
- Local branch to PR: GQL `repository(owner,name){ pullRequests(first:3, headRefName:$b, orderBy:{field:CREATED_AT,direction:DESC}) { nodes{number state url} } }`. `gh pr status --json` also works for the current branch.
- Needs: `${user}` from GQL `viewer { login }`.

### 1.2 PR description / overview page

EXT features: title edit, body edit, state (open, closed, merged, draft), author, labels, assignees, reviewers with state, milestone, projects, linked issues, commits list, full timeline (comments, reviews, force push, label, assign, review request, merge, auto-merge events, merge queue), reactions, checks, merge box, mergeability, "Changes since last review", webview refresh interval 60 s. Sources: [pullRequestOverview.ts](https://github.com/microsoft/vscode-pull-request-github/blob/main/src/github/pullRequestOverview.ts), fragments in [queriesShared.gql L48-L520](https://github.com/microsoft/vscode-pull-request-github/blob/main/src/github/queriesShared.gql).

gh way:
- Basic: `gh pr view N --json number,title,body,state,isDraft,author,labels,assignees,milestone,reviewRequests,latestReviews,reviews,comments,commits,files,additions,deletions,changedFiles,mergeable,mergeStateStatus,reviewDecision,statusCheckRollup,autoMergeRequest,closingIssuesReferences,baseRefName,headRefName,headRefOid,baseRefOid,headRepository,isCrossRepository,maintainerCanModify,projectItems,url`. Field list is from `gh pr view --json` help.
- Timeline: gh cannot do it. Use GQL `repository(owner,name){ pullRequest(number:$n){ timelineItems(last:150){ nodes { __typename ... on IssueComment{id author{login} body bodyHTML createdAt} ... on PullRequestReview{id state body author{login} submittedAt} ... on LabeledEvent{label{name color} actor{login} createdAt} ... on ReviewRequestedEvent{requestedReviewer{... on User{login}} } ... on HeadRefForcePushedEvent{beforeCommit{oid} afterCommit{oid}} ... on MergedEvent{commit{oid}} ... on AutoMergeEnabledEvent{enabler{login}} } } } }`. Same as EXT query `TimelineEvents`.
- Edit title and body: `gh pr edit N --title --body-file`, or GQL `updatePullRequest(input:{pullRequestId,title,body})`.
- Labels, assignees, reviewers, milestone: `gh pr edit N --add-label --remove-label --add-assignee --add-reviewer --milestone --add-project`. Reviewer teams use `org/team`.
- Add issue comment: `gh pr comment N --body-file -` or GQL `addComment(input:{subjectId,body})`. Edit comment: GQL `updateIssueComment`.
- Reactions: GQL `addReaction(input:{subjectId,content})` and `removeReaction`.
- Live refresh: GQL query `LatestUpdates(since)` in EXT. Cheap alternative: poll the REST `GET /repos/{o}/{r}/pulls/{n}` with ETag. See section 3.

### 1.3 Checkout of PR branch

EXT: "Checkout Pull Request" (`pr.pick`), by number, in worktree, on vscode.dev, on Codespaces, "Checkout Default Branch" (exit). Stores metadata in git config key `branch.<name>.github-pr-owner-number`. It fetches the PR head, adds a remote for fork PRs, and sets upstream. Settings: `pullPullRequestBranchBeforeCheckout`, `upstreamRemote`, `postDone` (checkout default branch), `deleteBranchAfterMerge`, `defaultDeletionMethod.*`. Source: [pullRequestGitHelper.ts](https://github.com/microsoft/vscode-pull-request-github/blob/main/src/github/pullRequestGitHelper.ts).

gh way:
- `gh pr checkout N [-b branch] [--force] [--detach] [--recurse-submodules]`. It handles forks and the config. Your editor can show the result by reading git state.
- Without gh: `git fetch origin pull/N/head:pr/N` then `git switch pr/N`. This works for same-repo and fork PRs. Push back to a fork needs the head repo URL from `headRepository` and `maintainerCanModify`.
- Worktree: `git worktree add` after the fetch. (`gh issue develop --worktree` exists for issues.)
- Match PR to branch: write the same git config key for the JetBrains-like "current branch has PR" display. Or cache by `headRefName` plus `headRepositoryOwner`.

### 1.4 Review mode

EXT behaviour: When a PR branch is checked out, "review mode" starts. `useReviewMode` = auto. Source: [reviewManager.ts](https://github.com/microsoft/vscode-pull-request-github/blob/main/src/view/reviewManager.ts), [prChangesTreeDataProvider.ts](https://github.com/microsoft/vscode-pull-request-github/blob/main/src/view/prChangesTreeDataProvider.ts), [reviewCommentController.ts](https://github.com/microsoft/vscode-pull-request-github/blob/main/src/view/reviewCommentController.ts).

a) Changed files tree: tree or flat (`fileListLayout`), status letter and colour, comment-count badge, "viewed" checkbox (synced with GitHub), `hideViewedFiles`, commits sub-tree, "changes since last review" (compare last reviewed commit with head). Next/previous diff command. Source: [changesSinceReview.ts](https://github.com/microsoft/vscode-pull-request-github/blob/main/src/github/changesSinceReview.ts) uses REST `repos.compareCommits`.
   gh way:
   - Files: `gh pr view N --json files` gives path, additions, deletions only (no status). Better: REST `GET /repos/{o}/{r}/pulls/{n}/files?per_page=100` (fields: filename, status, previous_filename, additions, deletions, patch, sha). Max 3000 files.
   - Viewed state: GQL `pullRequest(number){ files(first:100,after:$c){ nodes{path viewerViewedState} pageInfo{hasNextPage endCursor} } }`. Mutations `markFileAsViewed(input:{pullRequestId,path})`, `unmarkFileAsViewed`. No REST and no gh subcommand.
   - Commits: REST `GET /pulls/{n}/commits` or GQL `commits(first:50)`.
   - Since last review: GQL `LatestReviewCommit`: `pullRequest{ reviews(last:1,author:...){nodes{commit{oid}}} }` then `git diff <oid>..<headRefOid>` locally, or REST `GET /repos/{o}/{r}/compare/{base}...{head}`.

b) Diff with comments: EXT shows base against head in the diff editor. File content comes from local git after fetching the PR head and the merge base. It uses virtual documents when the branch is not checked out. Source: [fileChangeModel.ts](https://github.com/microsoft/vscode-pull-request-github/blob/main/src/view/fileChangeModel.ts), [gitContentProvider.ts](https://github.com/microsoft/vscode-pull-request-github/blob/main/src/view/gitContentProvider.ts).
   gh way: do not use `gh pr diff` for display. Use local git: `git fetch origin pull/N/head`, then `git merge-base`, then your existing DiffView with two revisions. This matches the "ideas" in MyGo DiffView and works for large files. `gh pr diff N --patch` is a fallback with no clone.
   Note: the diff "position" for old REST comments counts lines in the patch. GQL uses `line`, `startLine`, `diffSide` (LEFT or RIGHT). Prefer GQL fields.

c) Inline comment threads: widget in the gutter, "expandUnresolved" (`commentExpandState`), reply, edit, delete, reactions, outdated threads, "Diff comment with HEAD", copy comment link, file-level comments (`subjectType: FILE`).
   gh way (GQL only):
   ```graphql
   query($o:String!,$n:String!,$num:Int!,$after:String){
    repository(owner:$o,name:$n){ pullRequest(number:$num){
     reviewThreads(first:100,after:$after){ pageInfo{hasNextPage endCursor}
      nodes{ id isResolved isOutdated viewerCanResolve viewerCanUnresolve path diffSide line startLine originalLine originalStartLine subjectType
       comments(first:100){ nodes{ id databaseId url body bodyHTML author{login avatarUrl} diffHunk path position originalPosition state createdAt replyTo{databaseId} commit{oid} originalCommit{oid} viewerCanUpdate viewerCanDelete pullRequestReview{databaseId} } } } } } } }
   ```
   Fields match EXT fragments `ReviewThread` and `ReviewComment` ([queriesShared.gql L539, L768](https://github.com/microsoft/vscode-pull-request-github/blob/main/src/github/queriesShared.gql)).
   Mutations:
   - New thread: `addPullRequestReviewThread(input:{pullRequestId, pullRequestReviewId?, path, body, line, side, startLine, startSide, subjectType})`.
   - Reply: `addPullRequestReviewThreadReply(input:{pullRequestReviewThreadId, body, pullRequestReviewId?})` (EXT uses `addPullRequestReviewComment` with `inReplyTo`; both work, check the current schema).
   - Edit: `updatePullRequestReviewComment(input:{pullRequestReviewCommentId, body})`. Delete: REST `DELETE /repos/{o}/{r}/pulls/comments/{id}` (EXT uses `pulls.deleteReviewComment`).
   - Plain comments in the conversation (not in code) are issue comments (see 1.2).
   gh cannot create or read inline comments with a porcelain command. `gh pr view --json comments` shows only conversation comments; `reviews` shows review bodies only.

d) Suggestions: a "Make code suggestion" command inserts a fenced block with the language tag `suggestion` in the comment body. "Apply suggestion" applies the block to the local file. "Convert changes to PR suggestions" turns local edits into suggestion comments. Docs: [documentation/suggestAChange.md](https://github.com/microsoft/vscode-pull-request-github/blob/main/documentation/suggestAChange.md). Apply with Copilot is optional.
   gh way: a suggestion is only comment text. Build the body: "```suggestion\nnew lines\n```". Use the same thread mutations with `startLine` and `line` for the replaced range. To apply, edit the local file yourself (no gh need). GitHub web has "commit suggestion" with no GQL mutation; commit locally instead.

e) Pending review: Default comment type is `defaultCommentType: review`, so comments are added to a pending review. "Start review" then comments, then "Submit". EXT queries the pending review id with `GetPendingReviewId(pullRequestId, author)` and uses `addPullRequestReview`, `submitPullRequestReview`, `deletePullRequestReview`. Source: [queriesShared.gql L821, L1329-L1350](https://github.com/microsoft/vscode-pull-request-github/blob/main/src/github/queriesShared.gql).
   gh way: GQL.
   - Start: `mutation{ addPullRequestReview(input:{pullRequestId:$id}){ pullRequestReview{id} } }` (no event, so state is PENDING).
   - Find existing: `pullRequest{ reviews(states:PENDING, first:1, author? ){ nodes{id viewerDidAuthor} } }` (EXT passes the viewer login).
   - Add thread to review: use `pullRequestReviewId` in `addPullRequestReviewThread`.
   - Submit: `submitPullRequestReview(input:{pullRequestReviewId, event: APPROVE|REQUEST_CHANGES|COMMENT, body})`.
   - Discard: `deletePullRequestReview(input:{pullRequestReviewId})`.
   - Alternative one-shot REST: `POST /repos/{o}/{r}/pulls/{n}/reviews` with `event`, `body`, `comments[]` ({path, body, line, side, start_line, start_side}). Good for "submit all at once" from a local draft stored in your own state. This gives offline draft comments with low API use.

f) Approve, request changes, comment: `gh pr review N --approve|--request-changes|--comment -b "text"`. This is a body-only review. With pending comments use the GQL submit above. EXT "Approve on github.com" opens the browser for cases that need web (for example the user cannot approve own PR). Extra: "Ready, approve and auto-merge". EXT can also add an "attestation commit" (optional, `enableAttestationCommits`).

g) Resolve and unresolve threads: GQL `resolveReviewThread(input:{threadId})`, `unresolveReviewThread(input:{threadId})`. Check `viewerCanResolve`. "Comment and resolve" = reply then resolve. No gh subcommand.

h) Other review-mode items: "Open file on GitHub", "Compare base with PR head", "Compare PR head with local", show PR in status bar, "Close all PR editors", pre-review changes (Copilot, optional), "Collapse all comments", toggle commenting. These are UI features. Data comes from git.

### 1.5 Create PR flow

EXT: a webview in the side bar. Fields: base repo and branch, compare repo and branch, title, description, assignees, reviewers, labels, milestone, projects, "Create" or "Create draft", auto-merge create variants ("Create + auto-squash", and others), push branch if needed (`pushBranch`), fork creation if no push rights, title source (`pullRequestTitle`: ask, commit, branch, first commit...), description source (`pullRequestDescription`: template, commit, none...), default draft (`createDraft`), `labelCreated`, `assignCreated`, "generate title and description" with Copilot (optional), "revert PR". Views also show the compared files and commits. Source: [createPRViewProvider.ts](https://github.com/microsoft/vscode-pull-request-github/blob/main/src/github/createPRViewProvider.ts), [folderRepositoryManager.ts L1849 createPullRequest](https://github.com/microsoft/vscode-pull-request-github/blob/main/src/github/folderRepositoryManager.ts).

gh way:
- Create: `gh pr create --base B --head [owner:]branch --title T --body-file F --draft --reviewer a,org/team --assignee @me --label x --milestone m --project p --no-maintainer-edit`. Add `--fill`, `--fill-first`, or `--fill-verbose` for text from commits. It prints the PR URL. The command can push the branch (it prompts when no TTY? use `git push -u` yourself first, then pass `--head`).
- Native: GQL `createPullRequest(input:{repositoryId, baseRefName, headRefName, headRepositoryId, title, body, draft, maintainerCanModify})` ([queries.gql L715](https://github.com/microsoft/vscode-pull-request-github/blob/main/src/github/queries.gql)). Then `requestReviews(input:{pullRequestId, userIds, teamIds, union:true})`, `addLabelsToLabelable`, `addAssigneesToAssignable`, `updatePullRequest(input:{milestoneId})`, `addProjectV2ItemById`. gh does these in one command, so prefer gh for create when you want less code.
- Templates: GQL `repository(owner,name){ pullRequestTemplates{ body filename } }` (EXT queries only `body`). Also read `.github/pull_request_template.md` and `.github/PULL_REQUEST_TEMPLATE/*` locally. Note: gh `--template` exists to start from a template file; check `gh pr create --help`.
- Pickers: reviewers = GQL `repository.assignableUsers(first,after)` (EXT `GetAssignableUsers`), teams = `organization(login){ teams(first, after) { nodes{slug name} } }` (needs `read:org`), labels = REST `GET /repos/{o}/{r}/labels` or GQL `labels(first:100)`, milestones = GQL `milestones(states:OPEN)`, projects v2 = `repository.projectsV2` and `organization.projectsV2` (needs `project` scope; `gh auth refresh -s project`).
- Branch list for the base picker: GQL `refs(refPrefix:"refs/heads/", query:$q, first:50)` (EXT `ListBranches`).
- Compare view (files and commits before create): local `git diff base...head` and `git log base..head`. No API needed.
- Fork flow: REST `POST /repos/{o}/{r}/forks` (EXT `repos.createFork`), then add remote and push.
- Auto-merge after create: GQL `enablePullRequestAutoMerge(input:{pullRequestId, mergeMethod})` or `gh pr merge N --auto --squash`.
- Revert: `gh pr revert N` or GQL `revertPullRequest`.

### 1.6 Merge options

EXT: merge, squash, rebase menu; default `defaultMergeMethod`; commit title and message edit (uses repo settings `squashMergeCommitTitle` etc.); delete branch after merge; auto-merge enable and disable (`setAutoMerge`); merge queue enqueue and dequeue; "update branch" with base; ready for review; convert to draft; conflict resolution view; bypass or admin. EXT now calls a new async merge endpoint: `PUT /repos/{o}/{r}/pulls/{n}/merge-async` with header `X-GitHub-Api-Version: 2026-03-10` and body `{sha, merge_action, merge_method}` ([pullRequestModel.ts ~L609](https://github.com/microsoft/vscode-pull-request-github/blob/main/src/github/pullRequestModel.ts)). UNVERIFIED: public docs for this endpoint. The stable choice is REST `PUT /pulls/{n}/merge` or GQL `mergePullRequest(input:{pullRequestId, mergeMethod: MERGE|SQUASH|REBASE, commitHeadline, commitBody, expectedHeadOid})`.

gh way:
- `gh pr merge N --merge|--squash|--rebase [--auto] [--admin] [--delete-branch] [--subject T] [--body B] [--match-head-commit SHA]`. With a merge queue, no method is needed.
- Which methods are allowed: GQL `repository{ mergeCommitAllowed squashMergeAllowed rebaseMergeAllowed deleteBranchOnMerge viewerDefaultMergeMethod viewerPermission }`.
- Mergeability: GQL `pullRequest{ mergeable mergeStateStatus viewerCanEnableAutoMerge viewerCanDisableAutoMerge viewerCanUpdate isInMergeQueue mergeQueueEntry{state position} autoMergeRequest{mergeMethod enabledBy{login}} reviewDecision }`. Same in `gh pr view --json mergeable,mergeStateStatus,autoMergeRequest,reviewDecision`.
- Mark ready: `gh pr ready N`; draft again: `gh pr ready N --undo`.
- Update branch: `gh pr update-branch N [--rebase]`.
- Close, reopen: `gh pr close N [--comment] [--delete-branch]`, `gh pr reopen N`.
- Merge queue: GQL `enqueuePullRequest`, `dequeuePullRequest` ([queries.gql L752](https://github.com/microsoft/vscode-pull-request-github/blob/main/src/github/queries.gql)).
- gh cannot: resolve merge conflicts (use your local merge editor; MyGo See has conflict code ideas only).

### 1.7 CI status checks

EXT: status icon in tree, checks list in overview with required marks, links to details, view logs for GitHub Actions jobs ("checkRunLogContentProvider"), cancel and approve workflow runs, re-run (optional). Sources: [GetChecks query L1560](https://github.com/microsoft/vscode-pull-request-github/blob/main/src/github/queriesShared.gql), [githubRepository.ts L1585-L2309](https://github.com/microsoft/vscode-pull-request-github/blob/main/src/github/githubRepository.ts) (REST `actions.listJobsForWorkflowRun`, `downloadJobLogsForWorkflowRun`, `cancelWorkflowRun`, `approveWorkflowRun`, `checks.get`).

gh way:
- Status: `gh pr checks N --json name,state,bucket,link,workflow,event,startedAt,completedAt,description [--required]`. `bucket` is pass, fail, pending, skipping, cancel. `--watch -i 10` polls (do not use in the editor; poll yourself with ETag).
- Rich: the GQL `GetChecks` above. Use `commits(last:1){nodes{commit{statusCheckRollup{state contexts(first:100){nodes{__typename ... on CheckRun{name conclusion status detailsUrl isRequired(pullRequestNumber:$n) databaseId checkSuite{workflowRun{databaseId}}} ... on StatusContext{context state targetUrl isRequired(pullRequestNumber:$n)}}}}}}}`.
- Logs: `gh run view <run-id> --log-failed --job <job-id>` or REST `GET /repos/{o}/{r}/actions/jobs/{job_id}/logs` (302 to a download URL). Job id from REST `GET /actions/runs/{id}/jobs`. Checks from other apps have no logs in the API; open `detailsUrl`.
- Rerun: `gh run rerun <id> [--failed]`. Cancel: `gh run cancel <id>`.
- Cheap status polling: REST `GET /repos/{o}/{r}/commits/{sha}/status` and `/check-runs` with ETag (see 3.4).

### 1.8 Issues view and "start working on issue"

EXT: Issues tree with queries (`githubIssues.queries`: "My issues" grouped by milestone, "Created", "Recent"), issue overview webview, create issue from selection or TODO comments (code action for TODO, FIXME, HACK triggers), hover cards for #123 and @user, completion for #issue and @user, "Start working on issue" (create or checkout branch named `${user}/issue${issueNumber}` by `issueBranchTitle`, assign yourself by `assignWhenWorking`, status bar current issue, commit message template "${issueTitle}\nFixes #n"), "Stop working", "Continue working", "Go to linked code". Docs: [documentation/IssueFeatures.md](https://github.com/microsoft/vscode-pull-request-github/blob/main/documentation/IssueFeatures.md). Settings: `githubIssues.*` in package.json.

gh way:
- List: `gh issue list --search "<query>" --json number,title,state,labels,assignees,milestone,updatedAt,url`, or the GQL search above.
- View: `gh issue view N --json title,body,state,author,labels,assignees,milestone,comments,closedByPullRequestsReferences,url`.
- Create: `gh issue create --title --body-file --label --assignee --milestone --project`.
- Comment, edit, close, reopen: `gh issue comment|edit|close|reopen`.
- Start working: `gh issue edit N --add-assignee @me`, then `gh issue develop N --name <branch> --base <base> --checkout`. This creates a linked branch on GitHub (GQL `createLinkedBranch`). `--worktree path` exists in gh 2.105. Or local: `git switch -c <name>` using your template. EXT does the local way (and can link no branch). The linked branch gives the PR to issue link (`closingIssuesReferences`).
- Completion cache: users from `assignableUsers` or `mentionableUsers` (EXT `GetMentionableUsers`), issues from `GetPendingIssues`/`PullRequestNumbers` queries. Cache on disk per repo.
- Hover card for #n: GQL `IssuePreview`/`PullRequestPreview` (small queries in queriesShared.gql L976, L1007).

### 1.9 Notifications

EXT (experimental): tree "notifications:github", sorted by timestamp or priority (Copilot), mark as read, mark as done, open on GitHub, load more, status of PR notifications (`notifications` setting "pullRequests"), poll with the server interval. Source: [src/notifications/notificationsProvider.ts L96](https://github.com/microsoft/vscode-pull-request-github/blob/main/src/notifications/notificationsProvider.ts) and [notificationsManager.ts](https://github.com/microsoft/vscode-pull-request-github/blob/main/src/notifications/notificationsManager.ts).
gh way:
- `gh api notifications` (REST `GET /notifications?all=false&per_page=50`). Mark read: `PATCH /notifications/threads/{id}`. Mark done: `DELETE /notifications/threads/{id}`. Subscribe: `PUT /notifications/threads/{id}/subscription`.
- EXT reads `X-Poll-Interval` and `Last-Modified` from the response and sends `If-Modified-Since` ([notificationsManager.ts L231-L500](https://github.com/microsoft/vscode-pull-request-github/blob/main/src/notifications/notificationsManager.ts)). Do the same.
- `gh status` prints a text summary only; not machine readable.
- Limit: REST notifications work with classic token scope `notifications` or `repo`. The gh OAuth token has `repo`. UNVERIFIED: whether the gh token alone can read all notifications (the live token above has repo, read:org, gist, admin:public_key).

### 1.10 Copilot and coding agent (OPTIONAL)

EXT: "Copilot on my behalf" query, "Assign to coding agent" for an issue, delegate a task from chat, coding agent sessions view, "View session", `codingAgent.*` settings, "Summarize with Copilot", "Suggest a fix with Copilot", generate PR title and description, pre-review changes, apply suggestion with AI, notification priority with Copilot. Sources: [copilotRemoteAgent.ts](https://github.com/microsoft/vscode-pull-request-github/blob/main/src/github/copilotRemoteAgent.ts), [copilotApi.ts](https://github.com/microsoft/vscode-pull-request-github/blob/main/src/github/copilotApi.ts), [copilotPrWatcher.ts](https://github.com/microsoft/vscode-pull-request-github/blob/main/src/github/copilotPrWatcher.ts).
gh way: `gh agent-task` exists in recent gh (UNVERIFIED in 2.105; run `gh agent-task --help`). GQL: `suggestedActors(capabilities:[CAN_BE_ASSIGNED])` (EXT `GetSuggestedActors`) and `replaceActorsForAssignable` to assign the Copilot bot to an issue ([queries.gql L733, L768](https://github.com/microsoft/vscode-pull-request-github/blob/main/src/github/queries.gql)). Skip in phase 1 to 4. These features need no gh data apart from the PR list query with `author:copilot`.

### 1.11 Permalinks and links

EXT: copy GitHub permalink (commit SHA based, with line range), copy head link (branch), copy as Markdown, open permalink on GitHub, copy github.dev and vscode.dev links, copy PR link, copy comment link. EXT checks that a remote branch contains the commit and that the file is not modified. Source: [src/issues/util.ts L253-L460](https://github.com/microsoft/vscode-pull-request-github/blob/main/src/issues/util.ts), [uriHandler.ts](https://github.com/microsoft/vscode-pull-request-github/blob/main/src/uriHandler.ts) (open-PR URIs).
gh way: do it with local git only, no API. URL = `https://<host>/<owner>/<repo>/blob/<sha>/<path>#L10-L20`. Get the host and path from `git remote get-url`. `git rev-parse HEAD`; check `git branch -r --contains <sha>`. `gh browse path:10-20 --commit <sha> --no-browser` prints the URL as an option. Handle `git@host:o/r.git` and `ssh://` forms. 

### 1.12 Other EXT features (short)

- Terminal links: open PR URLs in the editor (`terminalLinksHandler`, `openPullLinks`).
- Source Control (SCM) integration: PR in the SCM view (`showInSCM`), quick diff against base (`quickDiff`).
- Stacked PRs: "Add to stack", stack summary and auto-merge ([pullRequestStack.ts](https://github.com/microsoft/vscode-pull-request-github/blob/main/src/github/pullRequestStack.ts)). Optional.
- Conflict resolution webview (new). Optional.
- Emoji, @ mention, #issue completion in comments.
- File upload to comments (images): REST upload is not public; EXT uses the web upload route. Skip.
- Tree decorations for PR status, avatars.

## 2. What gh cannot do (list)

1. Read or write inline review comments, threads, or pending reviews. Use `gh api graphql` or REST `/pulls/{n}/comments`, `/pulls/{n}/reviews`.
2. Resolve or unresolve a thread. GQL only.
3. Viewed-file state. GQL only.
4. Full PR timeline events. GQL (`timelineItems`) or REST `/issues/{n}/timeline`.
5. Required-check flags in one call (`gh pr checks --required` filters, but GQL `isRequired` gives the flag per check).
6. Notifications mark read or done (use `gh api` with PATCH and DELETE).
7. Mention and issue completion lists.
8. Merge queue enqueue (`gh pr merge` handles it implicitly).
9. Conflict resolution and "comment-in-diff" UI: local work.
10. `gh api` cache is time based (`--cache 60s`), not ETag based. It does not do 304 handling for you. UNVERIFIED how gh reports a 304 exit code; test before you rely on it. For ETag logic use your own HTTP client.

## 3. Auth, hosts, rate limits, polling

### 3.1 Detect user and token
- Machine readable: `gh auth status --json hosts` (gh 2.105 supports it; live output: `{"hosts":{"github.com":[{"state":"success","active":true,"host":"github.com","login":"...","tokenSource":"keyring","scopes":"repo, read:org, ...","gitProtocol":"ssh"}]}}`). Add `--hostname H`, `--active`, `--show-token`, `--no-refresh`. Exit code is 0 with --json. Without --json it exits 1 on a bad account. Source: `gh auth status --help` (local) and [gh manual](https://cli.github.com/manual/gh_auth_status).
- Token: `gh auth token --hostname H [--user LOGIN]`. Prints the token of the active account (or the chosen user). Auto-refreshes an expiring token unless `--no-refresh`. Source: [manual](https://cli.github.com/manual/gh_auth_token). Do not log the token. Do not write it to disk; hold it in memory and call `gh auth token` again on a 401.
- Env override: gh reads `GH_TOKEN`/`GITHUB_TOKEN` for github.com and `GH_ENTERPRISE_TOKEN`/`GITHUB_ENTERPRISE_TOKEN` for other hosts; `GH_HOST` sets the default host. Source: [gh help environment](https://cli.github.com/manual/gh_help_environment). `gh auth token` honours these.
- Find the gh binary: the GUI app on macOS does not inherit the shell PATH. Look in `/opt/homebrew/bin`, `/usr/local/bin`, or run a login shell (`zsh -lc 'command -v gh'`). See has a `shellpath` package for the same issue (idea only).
- Config files (read-only): `~/.config/gh/hosts.yml` lists hosts and users. Tokens are in the keyring, so do not parse the file for tokens.

### 3.2 Multi-account and Enterprise
- Per repo: pick the host from the git remote URL, then the account for that host: `gh auth status --json hosts` returns many users per host; the one with `active:true` is the default. Switch with `gh auth switch --hostname H --user U`; or pass `--user` to `gh auth token` without switching (no global state change).
- GHES base URLs: REST `https://HOST/api/v3`, GQL `https://HOST/api/graphql`. GHE.com (data residency) uses `https://api.SUBDOMAIN.ghe.com` and GQL `/graphql`. github.com uses `https://api.github.com` and `https://api.github.com/graphql`. EXT does the same mapping in [credentials.ts L709-L742](https://github.com/microsoft/vscode-pull-request-github/blob/main/src/github/credentials.ts). `gh api --hostname H` handles it for you.
- EXT keeps one github.com account plus one Enterprise account at a time ([README](https://github.com/microsoft/vscode-pull-request-github/blob/main/README.md)). You can do better: a map host to account.
- GHES versions are older. EXT avoids the `Bot` type and keeps "legacy" query variants for old servers (comments in [queries.gql L14](https://github.com/microsoft/vscode-pull-request-github/blob/main/src/github/queries.gql)). Read `GET /meta` on GHES for the version and keep a fallback query set.
- Scopes: EXT asks for `read:user user:email repo workflow`, and `project read:org` when needed ([credentials.ts L33-L35](https://github.com/microsoft/vscode-pull-request-github/blob/main/src/github/credentials.ts)). The gh default login has `repo read:org gist`. Missing `project`: `gh auth refresh -s project`. Missing `workflow` limits workflow-file push only. Check `X-OAuth-Scopes` in a response header, or the `scopes` field above.

### 3.3 Fallback when gh is missing
Order of choices:
1. Env token (`GH_TOKEN`, `GITHUB_TOKEN`).
2. `git credential fill` (protocol=https, host=H): gives the token that git uses (macOS keychain, GCM). Works for many users with HTTPS remotes. Use for REST and GQL only if it is a PAT or OAuth token (SSH remotes have none).
3. Own OAuth device flow: `POST https://github.com/login/device/code` then poll `/login/oauth/access_token` ([docs](https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/authorizing-oauth-apps#device-flow)). Needs an OAuth App client id (public, no secret). Store the token in macOS Keychain (`security` CLI or Security.framework via purego; MyGo is pure Go, so check how you reach Keychain without cgo).
4. Paste a fine-grained PAT. Store in Keychain.
5. Read-only mode: no token, public repos, 60 requests per hour per IP.
Offer "Install gh" and "Sign in" prompts. Also state clearly which features need which scopes.

### 3.4 Rate limits, caching, low CPU and network
Verified live on this machine with gh 2.105: `gh api rate_limit` shows core 5000 per hour, graphql 5000 points per hour, search 30 per minute. Response headers include `Etag`, `Last-Modified`, `X-Ratelimit-Remaining`, and `Cache-Control: private, max-age=60`.
Rules from official docs ([REST rate limits](https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api), [best practices](https://docs.github.com/en/rest/using-the-rest-api/best-practices-for-using-the-rest-api), [GraphQL limits](https://docs.github.com/en/graphql/overview/rate-limits-and-node-limits-for-the-graphql-api)):
- REST user limit is 5000 per hour (higher on GHE Cloud). GraphQL has a separate point budget of 5000 per hour; cost depends on node count. Ask for `rateLimit{cost remaining resetAt}` in each query (EXT adds a `RateLimit` fragment to every query).
- Search API: 30 per minute. The PR list uses it, so cache per query and refresh on demand, or with a slow timer (EXT default is no auto refresh).
- Conditional REST requests: send `If-None-Match: <etag>` (or `If-Modified-Since`). A 304 answer does not count against the primary limit when the request is authorised (docs: [conditional requests](https://docs.github.com/en/rest/using-the-rest-api/best-practices-for-using-the-rest-api#use-conditional-requests-if-appropriate)). Keep a small disk cache keyed by URL with ETag and body. 
- GraphQL has no ETag or 304 support (UNVERIFIED in official text; treat as true). To save cost for GraphQL, ask only for needed fields, use small `first:` values, and poll a cheap REST endpoint to know when to run the big query.
- Cheap change detectors: `GET /repos/{o}/{r}/pulls/{n}` ETag (changes when `updated_at` changes); `GET /repos/{o}/{r}/issues/{n}/timeline`; notifications `Last-Modified` plus `X-Poll-Interval` (default 60 s; obey it); `GET /repos/{o}/{r}/commits/{sha}/check-runs` for CI.
- Polling policy: poll only the visible PR, only when the window is focused, back off to 5 minutes when idle, stop on blur. Use `Retry-After` and `X-RateLimit-Reset` on 403 or 429 (secondary limits). Do not send mutations in parallel (docs advise serial writes).
- Avoid process spawn per call. A `gh` subprocess costs about 30 to 100 ms and loads config each time. Use one Go HTTP client with keep-alive and HTTP/2.
- Memory: keep only the data for open PRs; store thread lists as compact structs; load `patch` text on demand; use local git for diffs of large files (a 1 GB file never goes through the API; the REST file list also truncates patches above a size limit).
- Webhooks are not available for a desktop app. Use polling.
- Pagination: GQL cursors (`first:100`), REST `per_page=100` and `Link` header. `gh api --paginate --slurp` for gh.

## 4. See (wibus-wee/see) - ideas only (no licence)

Findings from the clone (no code copied or quoted beyond names):
- No GitHub API, no `gh` call, no OAuth, no PR or issue feature. A search for `api.github.com`, `graphql`, `octokit`, `gh auth`, and `exec.Command("gh")` found only website text.
- It has host-neutral Git UI: `internal/git/remote.go` has `WebURL` that converts a remote URL (ssh and https) to the web page of the repository; the README says "copy ... their link on GitHub, GitLab and similar hosts" ([README](https://github.com/wibus-wee/see#readme)).
- `internal/avatar` finds GitHub or Gravatar avatars from a commit email, with a one-week disk cache. It uses the `users.noreply.github.com` pattern.
- `internal/git` has blame, branch diff (`branch_diff.go`), conflict, history, stash, and index patch (staging hunks). It shows that a Git panel and branch compare can be built without any API. These are the local building blocks for the review mode of section 1.4.
- Idea to take: keep all diff, blame and permalink logic in a local git layer; add the GitHub layer on top as a separate package.

## 5. Suggested architecture

- Package `forge` (interface) with `github` implementation. Later GitLab. Keep the UI independent of the host.
- `github/auth`: gh detect, token get, fallback chain, per-host account map.
- `github/client`: HTTP client, REST and GQL calls, ETag disk cache, rate-limit tracker, retry. Use `encoding/json` with typed structs for only the fields you need.
- `github/model`: PR, Review, Thread, Comment, Check, Issue, Notification.
- Drafts: keep local drafts of comments in your own store, then submit in one REST review call (see 1.4e). This reduces calls and works offline.
- Reuse the editor DiffView and the gutter markers for review threads. Use comment widgets under lines.

## 6. Suggested build order

Phase 0 - Foundation (about 1 week)
- gh detect (`gh auth status --json hosts`), `gh auth token`, remote parsing, host to API URL map, HTTP client with ETag cache and rate-limit tracker. Show the user avatar and login in the status bar.

Phase 1 - Read-only PR list and overview
- Search queries list (default four categories), PR per current branch, PR overview (body, reviewers, labels, checks summary, timeline), open on GitHub, copy permalink (local git only). Search Everywhere entry "Pull requests" using a cached list.

Phase 2 - Checkout and local review
- Checkout (`gh pr checkout` or `git fetch pull/N/head`), changed-files tree with viewed marks, diff from local git (base to head, commit ranges, since last review), CI checks list with logs link, status icons.

Phase 3 - Comments and review
- Read threads (GQL), show in DiffView and gutter. Reply, new thread, edit, delete, reactions. Pending review with local drafts, submit (approve, request changes, comment). Resolve and unresolve. Suggestions (insert and apply).

Phase 4 - Create and merge
- Create PR dialog (title, body with template, base, draft, reviewers, labels, assignees, milestone), push and fork flow, merge (method, commit message, auto-merge, delete branch), ready for review, update branch, close and reopen, revert.

Phase 5 - Issues and notifications
- Issues list and view, create issue, "start working on issue" (`gh issue develop`), #123 and @user completion and hover, notification list with polling (`X-Poll-Interval`), mark read and done, command palette entries for all.

Phase 6 - Optional
- Merge queue, stacked PRs, Actions log viewer, rerun, conflict resolution UI, Copilot or agent features, GHES legacy query set, projects v2 picker, TODO to issue code action.

## 7. Open questions to test before you build
1. The 304 exit behaviour of `gh api -H 'If-None-Match: ...'` (use own HTTP client to avoid it).
2. `merge-async` endpoint is new in EXT; use `mergePullRequest` GQL for now.
3. Whether the gh OAuth token has enough scope for notifications, projects, and org teams on each user machine. Detect scopes and show a "grant" action (`gh auth refresh -s <scope>`).
4. Exact GQL schema names for thread reply mutations on GHES versions.
