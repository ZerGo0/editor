# Agent instructions

## UI and design

Read [docs/design.md](docs/design.md) before you change UI, and load the `design-system` skill (`.agents/skills/design-system/SKILL.md`).

- Build UI from `internal/design` components; take every visual value from `internal/design/token`. If a value or component is missing, add it there, not in the feature.
- Give every interactive element a label, a tooltip with its shortcut, and hover, pressed, focus-visible and disabled states. Prefer command IDs (`action.CommandIconButton`).
- Show loading, empty and error states. Keep dark, light and high contrast working.
- `go test ./internal/design` must pass. `TestDesignRules` rejects raw colors, raw numbers in visual setters, raw MyGo widgets and `c.Theme()` reads in feature packages. Use `//design:allow <reason>` only for a value an outside format dictates. The allowlist `internal/design/testdata/rules_allowlist.txt` only shrinks: `DESIGN_RULES_UPDATE=1 go test ./internal/design -run TestDesignRules`.
