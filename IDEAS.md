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
