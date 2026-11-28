---
name: editor-review
description: Ask the user to review your work in the Editor app and wait for the decision. Use when the user asks you to have them review code changes, a pull request, or a Markdown file (a plan, spec or doc) in the editor, or before you commit when the user wants to approve your changes first.
---

# Review in the Editor

The `editor` shell command opens a review tab in the Editor app. The user annotates lines and picks a decision. The command waits until then, prints the result on stdout and exits with 0. Exit 1 means the review could not start (stderr says why). Exit 2 means wrong arguments.

## Choose the command

| The user wants to review | Run |
| --- | --- |
| Your uncommitted changes (with new files) | `editor review` |
| Only staged, only unstaged, or the last commit | `editor review -diff staged` (or `unstaged`, `last-commit`) |
| Everything on this branch | `editor review -diff branch` (add `-base <branch>` for a stacked branch) |
| Another repository or worktree | `editor review <folder>` |
| A GitHub pull request | `editor review <pull request URL>` |
| A Markdown file, for feedback only | `editor annotate <file.md>` |
| A Markdown file that needs an approval | `editor annotate -gate -json <file.md>` |

Do not run `editor plan-hook` yourself. It is a Claude Code PermissionRequest hook for ExitPlanMode, and it opens by itself when you leave plan mode.

## Run it

- A review can take a long time. Run the command in the background with the longest timeout (`run_in_background` and `timeout: 7200000`), then wait for it to end. Do not kill it: a review that ends without a decision reads as no feedback.
- Do not run a second review while one is open.
- If a time limit stopped the command, run it again.

## Read the result

- `editor review` prints plain text, or with `-json` one line `{"decision": "approved" | "annotated" | "dismissed", "message": "..."}`. Decide only by `decision`, never by the message text.
- `editor annotate` prints nothing when the user closes the tab, "The user approved." on approval, and otherwise the feedback. With `-json` it prints `{"decision", "feedback", "annotationCount", "target"}`.
- **approved** with no notes: go on.
- **approved** with notes: go on, and treat the notes as guidance, not as a change request.
- **annotated**: address the feedback. The review message asks you to check each finding against the code and to give a verdict before you change code. Do that.
- **dismissed**: the user closed the review without feedback. Ask what they want next.
