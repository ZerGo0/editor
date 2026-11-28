# Editor

A native macOS code editor written in Go with [MyGo](https://github.com/egoist/mygo). It follows the VS Code workbench and adds ideas from JetBrains IDEs.

![Editor in the light and dark theme](docs/preview.png)

## Features

- Split editors with VS Code tab drag and drop
- Search Everywhere (press Shift twice), Go to File and Find in Files with multiline queries
- VS Code default keybindings, plus an import of your VS Code keybindings
- Git source control, multi-file diffs, and GitHub pull requests and issues
- Local history, hot exit for unsaved files and Open Recent
- Light, dark and system themes
- Annotated review for coding agents

## Review with an agent

A coding agent can ask you to review its work in the editor. The agent runs one of the commands below and waits. The editor opens a review tab in the project window. Click a line number, or the **+** beside it, to leave a comment or suggest code on that line. You can also add file comments and global comments. Then choose a decision:

- **Approve**: no annotations, the work is accepted.
- **Send Feedback**: the agent gets your annotations to address.
- **Approve with Notes**: the work is accepted, and the agent gets your annotations as non-blocking notes.
- **Close**: the agent is told that the review closed without feedback.

The command prints the result for the agent and exits with 0. It exits with 1 when the review cannot start, and with 2 for wrong arguments.

The commands come from the `editor` shell command. Run **Install 'editor' Command in PATH** from the Command Palette. If you installed the command before, install it again to get these commands.

- `editor review [-json] [-diff <mode>] [-base <ref>] [folder]` reviews local changes. Modes: `uncommitted` (the default: the working tree against HEAD, with new files), `staged`, `unstaged`, `last-commit` and `branch` (the working tree against the merge base of `-base`, by default the default branch). You can change the mode in the review tab.
- `editor review <pull request URL>` reviews a GitHub pull request. Your feedback goes to the agent, not to GitHub.
- `editor annotate [-gate] [-json] [-hook] <file.md>` annotates a Markdown file on its source lines. Without `-gate`, there is no Approve. `-hook` prints a hook decision that blocks with your feedback.
- `editor plan-hook` reviews a Claude Code plan. Approve lets Claude Code continue with the plan, and Send Feedback sends it back to plan mode with your annotations.

To teach Claude Code when and how to ask for a review, install the skill from this repository:

```sh
mkdir -p ~/.claude/skills && cp -R skills/editor-review ~/.claude/skills/
```

To review every Claude Code plan, add the hook to `~/.claude/settings.json` or `.claude/settings.json`. The long timeout gives you time to review:

```json
{
  "hooks": {
    "PermissionRequest": [
      {
        "matcher": "ExitPlanMode",
        "hooks": [{"type": "command", "command": "editor plan-hook", "timeout": 345600}]
      }
    ]
  }
}
```

## Build

Needs macOS on Apple silicon and Go 1.27.4 or later.

```sh
go tool mygo build
open build/darwin-arm64/Editor.app
```

## Develop

The [Taskfile](Taskfile.yml) lists the common commands:

```sh
task dev       # run with live reload
task check     # vet, format check and unit tests
task test:e2e  # end-to-end tests against a built app
task preview   # capture docs/preview.png again
```

Read [AGENTS.md](AGENTS.md) and [docs/design.md](docs/design.md) before you change the UI.
