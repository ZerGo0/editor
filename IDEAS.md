# Ideas

Features we chose not to build now. Each one comes from the [IDE research](docs/research/ide-features.md). Effort: S small, M medium, L large.

## Code intelligence

- **Completion and definitions (M):** word completion, and go to definition and find usages from the symbol index.
- **LSP client (L):** language servers, gopls first, then TypeScript, Python and Rust. Gives completion, rename, hover and refactorings. Changes the "no LSP" decision.
- **Snippets (M):** VS Code snippet format with tab stops. The MyGo editor already has the engine.
- **Outline and inline errors (S to M):** an Outline side view, sticky scroll, and diagnostics shown at the end of the line (Error Lens style).

## Run and tools

- **Terminal (L):** an integrated terminal in the bottom panel. The main reason people stay in VS Code.
- **Run and tasks (L):** a Run button, tasks (tasks.json style) and an output panel with clickable errors. The base for a test runner.
- **Formatters (M):** Format Document and format on save with external formatters such as gofmt, prettier and ruff.

## Comfort

- **Themes (M):** color themes, with import of VS Code themes.
- **Background work list (S):** a list of background jobs with the reason and a Cancel button, plus visible startup and idle CPU budgets.

## Extras

- **Zen mode (S):** full-screen editing without distractions.

## Review

- **Annotated review for agents (L):** owner request, modeled on [Plannotator](https://plannotator.ai/). Annotate local changes (PR-style diff with a file tree and staging), Markdown plans (rendered preview, with versions and diffs between them), GitHub PR diffs and any open file. Annotation types as in Plannotator: comments, replacements or code suggestions, deletions, and a global comment. The finished review goes straight to a running agent, for example Claude Code through a hook or a watched file, as structured feedback. When we build it, read how Plannotator sends feedback to each agent in its code ([backnotprop/plannotator](https://github.com/backnotprop/plannotator), MIT or Apache 2.0).
