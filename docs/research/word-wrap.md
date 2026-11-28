# Word wrap: VS Code compared with MyGo

Sources: `src/vs/editor/common/viewModel/monospaceLineBreaksComputer.ts` and `src/vs/editor/common/config/editorOptions.ts` in [microsoft/vscode](https://github.com/microsoft/vscode) (main, read 2026-11-28), and `ui/editor/view.go` in MyGo v0.10.0.

## VS Code

### Settings

| Setting | Values | Default |
|---|---|---|
| `editor.wordWrap` | `off`, `on`, `wordWrapColumn`, `bounded` | `off` |
| `editor.wordWrapColumn` | 1 or more | 80 |
| `editor.wrappingIndent` | `none`, `same`, `indent`, `deepIndent` | `same` |
| `editor.wrappingStrategy` | `simple` (columns), `advanced` (browser layout) | `simple` |
| `editor.wordBreak` | `normal`, `keepAll` (no breaks inside CJK text) | `normal` |
| `editor.wordWrapBreakBeforeCharacters` | `([{'"〈《「『【〔([{「£¥$£¥++` | |
| `editor.wordWrapBreakAfterCharacters` | space, tab, `})]?|/&.,;¢°'''‰°C`, CJK punctuation and small kana | |
| `diffEditor.wordWrap` | `off`, `on`, `inherit` | `inherit` |
| `editor.wrapOnEscapedLineFeeds` | a literal `\n` in a string line also breaks | `false` |

### Wrapping column

- `off`: no wrap.
- `on`: the viewport column: `floor((contentWidth - verticalScrollbarWidth - 2) / typicalHalfwidthCharacterWidth)`, less 1 when the wrap indicator shows.
- `bounded`: `min(viewport column, wordWrapColumn)`.
- `wordWrapColumn`: exactly `wordWrapColumn`, also when the viewport is narrower (the text then scrolls horizontally).
- Toggle Word Wrap (Alt+Z, `toggleWordWrap.ts`) keeps a transient state for each file. With no state it sets `wordWrapOverride2` to `off` when the file wraps and to `on` when it does not. With a state it removes it, so the file goes back to the setting (for example back to `wordWrapColumn`). In a diff editor it sets the same state on both sides.

### Break rules (simple strategy)

- Widths are in columns: a tab goes to the next tab stop, a full-width character takes `typicalFullwidthCharacterWidth / typicalHalfwidthCharacterWidth` columns, a control character takes the full-width count, a surrogate pair takes 2 and is never split, every other character takes 1.
- Each character has one class: break after (the list above), break before (the list above), ideographic (U+3040-U+30FF, U+3400-U+4DBF, U+4E00-U+9FFF), or none.
- A break is allowed before a character when it is not a space and one of these is true: the previous character is break-after and this one is not; this one is break-before and the previous one is not; the previous one is ideographic and this one is not break-after; this one is ideographic and the previous one is not break-before. With `keepAll` the two ideographic rules are off.
- So spaces stay at the end of the row, a run of break-after characters stays together, and VS Code does **not** break between a word and other characters: `foo-bar` and `a=b` never break inside, `foo.bar` breaks after the dot.
- When a character goes past the column, the row ends at the last allowed break. If there is none, or the rest of the row after that break would not fit in a continuation row, the row ends before that character (a forced break). Each row has at least one character.

### Wrapped-line indent

- `none`: 0. `same`: the visible width of the leading whitespace. `indent` and `deepIndent`: that width plus 1 or 2 tab stops.
- If the indent plus one full-width character is more than the column, the indent is 0. There is no other cap.
- Continuation rows wrap at `column - indent`.

## MyGo v0.10.0

- `State.SoftWrap` wraps at the editor width, or at `WrapColumn` when that is narrower: VS Code's `bounded`. There is no `wordWrapColumn` mode that wraps past the viewport.
- Widths are pixel advances of the shaped characters, not columns.
- Breaks follow Zed's `LineWrapper`: after whitespace, between a word and a non-word character, anywhere in CJK text with kinsoku rules. So `foo-bar` and `a=b` can break where VS Code does not.
- The continuation indent is the leading spaces only (tabs are not counted), capped at half the wrap width. There is no `none`, `indent` or `deepIndent`.
- The break code (`view.wrapBreaks`, `rowEnd`, `fitRow`, `wrapIndent`, `canBreak`) is not exported, and `editor.State` has no hook for it.

## What the change needs

The editor cannot match VS Code from outside MyGo. It needs one of:

1. A fork of MyGo (a `replace` directive to a copy in the repository) with a wrap hook in `editor.State`, for example a `Wrap` option that gives the column mode, the indent mode and the break classes, or a function that returns the break offsets and indent of a line.
2. An upstream change in MyGo that adds such a hook.

With a hook, the editor implements the VS Code algorithm above in Go (`internal/wordwrap`), with unit tests built from the VS Code test cases (`monospaceLineBreaksComputer.test.ts`), and maps `editor.wordWrap`, `editor.wordWrapColumn`, `editor.wrappingIndent` and `diffEditor.wordWrap` to it.
