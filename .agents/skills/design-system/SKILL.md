---
name: design-system
description: Build or change editor UI with the internal/design package. Load before any UI work in this repository, including new views, layout changes, states, colors and copy.
---

# Design system

`docs/design.md` is the reference: tokens with values, every component, and the rules. This skill is the working loop.

## Pick the parts

- Views in the sidebar: `pane.ViewTitle`, `pane.Stack` or `pane.Section`, then `list.View` or `list.Tree`. Put text forms in `design.ViewBody`.
- Pages in the editor area (settings, details): `design.Page`, `design.Form`, `design.Setting`.
- Actions: `action.CommandIconButton` when a command exists, else `action.IconButton` with a label and keys. Text actions: `action.Button` (one `Primary` per surface at most), `action.SplitButton`.
- Inputs: `field.TextInput`, `field.TextArea`, `field.SearchInput` with `field.Toggle`, `action.Select`, `action.Checkbox`, `action.Switch`.
- Chrome: `chrome.TabStrip`, `chrome.Breadcrumbs`, `chrome.StatusItem`, `chrome.CommandCenter`, `chrome.NavButton`, `chrome.ActivityBar`.
- States: `feedback.Loading`, `feedback.Empty`, `feedback.Error`, `feedback.ProgressBar` for busy views, `feedback.Notice` inline, `feedback.Toast` for results.
- Small parts: `badge.Count`, `badge.Tag`, `badge.Decoration`, `badge.Avatar`, `badge.Keys`, `design.Text` with a `token.TextStyle`.
- Pickers: `quickinput.View`.
- Layout glue: `design.Stack`, `design.Inline`, `design.Spacer` with semantic gaps (`token.GapInline`, `token.GapControl`, `token.GapStack`, `token.GapSection`).

Pass meaning, not looks: a variant, a `token.Tone`, a `token.GitStatus`, a command ID. If no component fits and the pattern appears twice, add one to `internal/design` with a gallery entry (`gallery.go`, `gallery_parts.go`) and a tester test.

## Rules that the build checks

- No `ui.RGB`, `ui.Hex`, `ui.Color{}` or `c.Theme()` in feature code: use `token.Of(c)`.
- No numbers or numeric constants in `Padding`, `Gap`, `Height`, `Width`, `FontSize`, `Radius` and the other visual setters: use tokens.
- No raw MyGo widgets (`ui.Button`, `ui.TextInput`, `ui.ButtonBase`, ...): use the design component.
- A rare exception needs `//design:allow <reason>` on or above the line.

## Before you finish UI work

1. `go test ./internal/design/...` passes, including `TestDesignRules`. If you removed violations, shrink the allowlist with `DESIGN_RULES_UPDATE=1 go test ./internal/design -run TestDesignRules`.
2. Every new interactive element has a label, a tooltip with keys where a shortcut exists, and works with Tab, Space, Enter and Escape. Check with `ui.Tester` (`AccessByRole`, `Key`).
3. The view shows loading, empty and error states, and disabled states where actions do not apply.
4. Copy uses VS Code wording, title case for commands and buttons, sentence case for messages, no final period on labels.
5. Look at it in both themes. For components: `DESIGN_SHOTS=<dir> go test ./internal/design -run 'Gallery|QuickInput'`. For the app, only when the user allows running it: the e2e harness `App.Screenshot` writes PNGs when `EDITOR_E2E_ARTIFACTS` is set (see `e2e/harness_test.go`); capture dark and light.
6. `gofmt`, `go vet` and `go tool mygo vet .` are clean.
