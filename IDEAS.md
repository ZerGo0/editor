# Ideas

Features we chose not to build now. Effort: S small, M medium, L large.

## Code intelligence

- **Completion and definitions (M):** word completion, and go to definition and find usages from the symbol index.
- **LSP client (L):** language servers, gopls first, then TypeScript, Python and Rust. Gives completion, rename, hover and refactorings. Changes the "no LSP" decision.
- **Code actions and formatting (M):** Quick Fix (Alt+Enter), Format Document and Format Selection (F17 in the owner's keys), through the LSP client or a formatter command (gofmt, prettier) where no server formats.
- **Snippets (M):** VS Code snippet format with tab stops. The MyGo editor already has the engine.
- **Outline and inline errors (S to M):** an Outline side view, sticky scroll, and diagnostics shown at the end of the line (Error Lens style).

## Run and tools

- **Terminal (L):** an integrated terminal in the bottom panel. The main reason people stay in VS Code. The owner's F7, Ctrl+V, Cmd+C and Shift+Enter terminal keys need it, with `workbench.action.terminal.sendSequence` and `workbench.action.focusActiveEditorGroup`.
- **Run and tasks (L):** a Run button, tasks (tasks.json style) and an output panel with clickable errors. The base for a test runner.
- **Formatters (M):** Format Document and format on save with external formatters such as gofmt, prettier and ruff.

## Comfort

- **Themes (M):** color themes, with import of VS Code themes.
- **Background work list (S):** a list of background jobs with the reason and a Cancel button, plus visible startup and idle CPU budgets.

## Extras

- **Zen mode (S):** full-screen editing without distractions.
- **Review over MCP (M):** an `editor mcp` stdio server with `review` and `annotate` tools and a review skill (MCP Skills extension), so agents other than Claude Code can ask for a review. Wait until MCP tasks (long-running tool calls) are stable in the clients and the Go SDK: the CLI, the ExitPlanMode hook and a skill file cover Claude Code today.
