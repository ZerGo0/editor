# Design system

The editor UI follows VS Code's workbench (Dark Modern and Light Modern) with macOS touches. Everything visual comes from `internal/design`. Feature code says *what* a thing is; the design packages decide how it looks.

## Build a view in 5 lines

A sidebar view with a title, a toolbar, a list and an empty state:

```go
pane.ViewTitle(c, "Source Control", pane.TitleOptions{Actions: func(c *ui.Context) {
	action.CommandIconButton(c, icons.Refresh, "git.refresh")
}})
list.View(c, &v.list, len(v.files), list.ViewOptions{Label: "Changes", Empty: func(c *ui.Context) {
	feedback.Empty(c, feedback.EmptyOptions{Icon: icons.GitBranch, Title: "No changes"})
}}, func(i int) list.Item { return list.Item{Label: v.files[i].Name, Icon: icons.File, Git: v.files[i].Status} })
```

A settings page:

```go
design.Page(c, "Editor", func() {
	design.Setting(c, "Word Wrap", "Controls how lines should wrap.", func() {
		action.Switch(c, "Word Wrap", &s.wrap, action.ChoiceOptions{})
	})
})
```

Stacked, collapsible, resizable sections:

```go
pane.Stack(c, &v.panes, []pane.Pane{
	{ID: "changes", Title: "Changes", Count: len(v.files), Body: v.changes},
	{ID: "stashes", Title: "Stashes", Body: v.stashes},
})
```

A quick pick:

```go
if r := quickinput.View(c, "Go to File", &v.pick, items, quickinput.Options{Placeholder: "Search files by name"}); r.Accepted >= 0 {
	v.open(items[r.Accepted])
} else if r.Closed {
	v.picking = false
}
```

Theme and commands, once at the window root:

```go
design.Apply(c, design.Settings{Appearance: design.AppearanceSystem, SystemAccent: true})
action.SetCommands(commandSource)
```

## Rules

### MUST

1. Take every visual value from `internal/design/token`: color roles from `token.Of(c)`, and spacing, sizes, radii, strokes, opacity, type, motion, elevation and layers from the named tokens. If a value is missing, add a token there. Do not write a literal.
2. Build buttons, inputs, rows, lists, trees, section headers, view titles, toolbars, tabs, status items, badges, empty, loading and error states, notices, menus and quick picks with the design components. Do not draw them with `ui.ButtonBase`, `ui.TextInput`, `ui.Row` and colors.
3. Give every interactive element a label. Icon-only controls get a tooltip; when a shortcut exists, the tooltip shows it (`Label (⌘K)`). Prefer command IDs (`action.CommandIconButton`, `action.CommandItem`): they get the title, keys and enabled state from the command registry.
4. Show hover, pressed, focus-visible and disabled states. The components do this; custom elements must match them.
5. Support dark, light, high contrast and the system accent. Never branch on the appearance in feature code.
6. Show a loading, empty and error state for every view that loads data (`feedback.Loading`, `feedback.Empty`, `feedback.Error`). Never block the UI thread.
7. Keep text at WCAG AA: 4.5:1 for text, 3:1 for icons, focus rings and large text. The palette tests enforce it for every role.
8. Do not animate. Every change shows on the next frame: no fades, slides, eases or element transitions. `design.Apply` turns off MyGo's built-in motion in every frame, and `TestNoMotion` fails on `Animate`, `AnimateTo`, `AnimateColor`, `Keyframes`, `Spring` and `Transition`. Only progress indicators and the caret blink move.
9. Keep `go test ./internal/design` green. `TestDesignRules` fails on raw colors, raw numbers in visual setters, raw MyGo widgets and direct theme reads in feature packages.

### SHOULD

1. Use VS Code's wording where VS Code has the same feature ("Stage Changes", "Go to File...", "Collapse Folders in Explorer").
2. Write UI text in title case for commands, buttons and menu items, and sentence case for descriptions, messages and placeholders. Keep labels short and without a final period. Use "..." on actions that ask for more input.
3. Keep density compact: 22 px rows and headers, 35 px titles and tabs, 16 px icons, 13 px body text.
4. Align content to the 4 px grid. Indent tree levels by `token.IndentStep`. Align view content to `token.PadView`.
5. Show row and section actions on hover and on keyboard focus only.
6. Put one primary button on a surface at most. Use ghost buttons in dense areas.
7. Add a component to `internal/design` when a pattern shows up in two features, or when it needs states, labels or keyboard handling. Add it with a gallery entry and a test.

### Escape hatch

When a literal is truly needed (for example, a value that a file format dictates), put `//design:allow <reason>` on the same line or the line above. The reason is required. `TestDesignRules` prints each violation with its file, line and fix.

## Principles

- **Meaning in, look out.** Callers pass a variant, a tone (`token.ToneError`), a git status or a command ID. Components choose colors, sizes and states.
- **Correct by construction.** Tokens are typed (`token.Space`, `token.Size`, `token.Radius`, `token.Stroke`, `token.Opacity`, `token.TextStyle`, `token.Layer`, `token.Elevation`). They have no exported constructors, so a raw number does not compile where a token is expected. Components that need a label take it as a parameter.
- **Theme from context.** `token.Of(c)` returns the palette of the theme in use, so no component takes a theme argument.
- **VS Code first, macOS polish.** Metrics and colors follow VS Code. Native touches: system accent, high contrast, popover menus, inset title bar. No motion.

## Tokens

The primitive scale (`token.Space2`...`token.Space32`, `token.Radius2`...`token.RadiusFull`, `token.Stroke1`, `token.Stroke2`) exists for the design packages. Feature code uses the semantic roles below.

### Spacing (`token.Space`)

| Token | Value | Use |
| --- | --- | --- |
| GapTight | 2 | Icon buttons in a toolbar, keycaps in a chord |
| GapInline | 4 | Icon to label, chips |
| GapRowIcon | 6 | Icon to label in rows, tabs, label to description |
| GapControl | 8 | Between buttons and controls |
| GapStack | 8 | Between rows of a form or a stack |
| GapGroup | 12 | Between groups in a view |
| GapSection | 16 | Between sections of a page |
| PadRow | 8 | Row and header side padding |
| PadView | 20 | View content inset (sidebar views, pages) |
| PadPanel | 12 | Panel and dialog padding |
| PadPopover | 4 | Popover, menu and quick input padding |
| PadControlX / PadControlEnd | 8 / 4 | Select and small button sides |
| PadButtonX | 12 | Button sides |
| PadInputX / PadInputY / PadInputEnd | 6 / 4 / 2 | Text input insets |
| PadBadgeX / PadChipX / PadKeyX | 4 / 6 / 4 | Badge, tag and keycap sides |
| PadNoticeX / PadNoticeY | 8 / 6 | Notice insets |
| PadStatusItemX | 6 | Status bar item sides |
| PadTabX | 12 | Tab start |
| PadEmpty / PadLoading | 24 / 12 | Empty and loading state insets |
| IndentStep | 8 | Tree indent per level |
| OffsetPopover / OffsetQuickOpen | 2 / 4 | Gap below an anchor, below the title bar |

### Sizes (`token.Size`)

| Token | Value | Token | Value |
| --- | --- | --- | --- |
| RowHeight | 22 | IconSize | 16 |
| HeaderHeight | 22 | IconSizeSmall | 14 |
| ViewTitleHeight | 35 | IconSizeGlyph | 12 |
| TabHeight | 35 | IconSizeLarge | 32 |
| TitleBarHeight | 35 | IconButtonSize | 22 |
| StatusBarHeight | 22 | ToggleSize | 20 |
| ToolbarHeight | 28 | CheckboxSize | 16 |
| ControlHeight | 26 | SwitchWidth / Height / Knob | 28 / 16 / 12 |
| ControlHeightSmall | 22 | SplitArrowWidth | 24 |
| HitTargetMin | 22 | ActivityItem | 32 |
| AvatarSize | 20 | BadgeHeight / BadgeSmall | 18 / 14 |
| TagHeight | 18 | KeyCapHeight / KeyCapMinWidth | 18 / 18 |
| DecorationWidth | 16 | DirtyDotSize | 8 |
| TwistieWidth | 16 | SashSize | 4 |
| ScrollbarSize | 10 | ProgressHeight | 2 |
| PaneMinHeight | 66 | SidebarMinWidth | 170 |
| PanelMinHeight | 100 | TabMinWidth | 80 |
| PopoverMaxWidth | 320 | DialogWidth | 440 |
| QuickInputWidth | 600 | CommandCenterWidth / MinWidth | 600 / 160 |
| EmptyStateWidth | 280 | NoticeMaxWidth | 480 |
| TooltipMaxWidth | 300 | QuickInputRows (count) | 20 |

### Radii, strokes, opacity

| Token | Value | Use |
| --- | --- | --- |
| RadiusNone | 0 | Bars, tabs |
| RadiusInline | 2 | Checkboxes, inline toggles, menu rows |
| RadiusControl | 4 | Buttons, inputs, icon buttons, rows, keycaps |
| RadiusCard / RadiusPopover / RadiusDialog | 6 | Cards, menus, quick input, dialogs, command center |
| RadiusPill | full | Badges, tags, avatars, switches |
| BorderWidth / SeparatorWidth | 1 | Borders, separators |
| FocusRingWidth / FocusRingOffset / FocusRingOffsetOuter | 1 / 1 / 2 | Focus ring |
| IndicatorWidth / IndentGuideWidth | 1 | Active tab and activity indicators, tree guides |
| KeyCapBottomWidth | 2 | Keycap bottom edge |
| OpacityHidden / OpacityDisabled / OpacitySeparator | 0 / 0.4 / 0.4 | Hidden actions, disabled, split separator |
| OpacityDragGhost / OpacityTintFill / OpacityTintBorder | 0.6 / 0.18 / 0.6 | Drag image, tag fill and border |

### Type (`token.TextStyle`)

| Style | Size | Weight | Notes |
| --- | --- | --- | --- |
| TextBody | 13 | 400 | Default UI text |
| TextStrong | 13 | 600 | Setting titles, emphasis |
| TextLabel | 12 | 400 | Descriptions, status bar, inputs' errors |
| TextParagraph | 12 | 400 | Muted, line height 1.4 |
| TextCaption | 11 | 400 | Muted captions |
| TextTitle | 11 | 700 | Uppercase view and section titles |
| TextHeading | 15 | 600 | Page and dialog headings |
| TextBadge | 11 | 600 | Count badges |
| TextMono | 12 | 400 | Menlo, code and hashes |

Use `design.Text(c, s, token.TextCaption)` in feature code. It applies the size, weight, case, family and the right text color.

### Delays, depth and layers

| Token | Value |
| --- | --- |
| TooltipDelay / EditorHover / SpinnerDelay / LongPress | 400 / 300 / 150 / 400 ms (delays, not animations) |
| ElevationPopover / ElevationDrag / ElevationDialog | y 2 blur 8 / y 2 blur 8 / y 4 blur 16, palette shadow color |
| LayerBase / Sticky / DropTarget / Popover / QuickInput / Toast / Tooltip | 0 / 10 / 20 / 100 / 200 / 300 / 400 |

### Colors (`token.Of(c)`)

| Group | Roles | Dark Modern | Light Modern |
| --- | --- | --- | --- |
| Layer | TitleBar, ActivityBar, SideBar, Panel, StatusBar, TabBar | #181818 | #F8F8F8 |
| Layer | Editor | #1F1F1F | #FFFFFF |
| Layer | Input | #313131 | #FFFFFF |
| Text | Primary, Secondary, Muted, Disabled, Link, Selection | #CCCCCC, #9D9D9D... | #3B3B3B, #616161... |
| Accent | Base, Hover, Text, Focus | #0078D4 | #005FB8 |
| List | Hover, ActiveSelection, InactiveSelection, Highlight, PickSelection, IndentGuide, Drop | #2A2D2E, #04395E, #37373D | #F2F2F2, #E8E8E8 |
| Control, Border, Tab, Bar, Menu, Badge, Key | Component states | VS Code values | VS Code values |
| Severity | Error, Warning, Info, Success and their notice backgrounds | #F48771, #CCA700, #3794FF, #3FB950 | #C72E0F, #855F00, #0066BF, #2E7D32 |
| Git | Added, Modified, Deleted, Renamed, Untracked, Ignored, Conflict, Submodule | VS Code, lifted for AA | VS Code, darkened for AA |
| Diff | Inserted/Removed line and text, gutter colors | VS Code | VS Code |

The exact values live in `token/dark.go` and `token/light.go`. `token.For` derives accent roles from the macOS accent color and raises contrast for high contrast mode. It then nudges any role below AA until it passes. Use `p.Tone(t)` and `p.GitColor(s)` instead of picking severity or git fields by hand.

## Components

All components take `c *ui.Context` first, then the label, then any bound value, then data, then an options struct whose zero value is the right default. Each returns `ui.Element` (chain layout such as `.Grow(1)` on it; do not restyle it).

### action

- `Button(c, label, ButtonOptions{Variant, Icon, Keys, Tooltip, Disabled, Small, ID})`: Secondary (default), Primary, Ghost.
- `IconButton(c, icon, label, IconOptions{Keys, Tooltip, Active, Disabled, KeepFocus, Tone, ID})`, `MenuIconButton(..., menu)`, `ToggleIconButton(c, icon, label, &on, IconOptions{})`. `KeepFocus` leaves the focus in the field the button acts on.
- `ExpandButton(c, label, expanded, IconOptions{})`: a chevron as high as its row that shows or hides a part, such as Toggle Replace in the find widget.
- `SplitButton(c, label, SplitOptions{..., Menu})`: the main button plus a chevron menu.
- `Checkbox(c, label, &checked, ChoiceOptions{Description, Disabled, ID})`, `Switch(c, label, &on, ChoiceOptions{})`.
- `Select(c, label, &value, []Option[T]{{Value, Label}}, SelectOptions{Disabled, ID, Width})`.
- `Item(m, label, ItemOptions{Icon, Mods, Key, Detail, Checked, Disabled, Danger, ID})`: a menu item.
- Commands: `SetCommands(src)`, `Lookup(id)`, `Run(id)`, `CommandIconButton(c, icon, id)`, `CommandButton(c, id, variant)`, `CommandItem(m, id)`. An unknown ID shows its ID and is disabled.

```go
if action.Button(c, "Commit", action.ButtonOptions{Variant: action.Primary, Keys: "⌘↩"}).Clicked() { v.commit() }
```

### field

- `TextInput(c, label, &value, InputOptions{Placeholder, Error, Icon, Toggles, Disabled, ReadOnly, AutoFocus, ID})`.
- `TextArea(c, label, &value, AreaOptions{..., MinLines, MaxLines})`: grows with its text.
- `SearchInput(c, label, &query, SearchOptions{..., Toggles, NoIcon})`: clear button, Escape clears.
- `Toggle{Icon, Label, Keys, On}`: inline toggles such as Match Case. The error shows below the box and is announced.

### list

- `View(c, &state, n, ViewOptions{Label, Empty}, func(i) Item)` and `Tree(c, &outline, roots, children, ViewOptions{}, func(k) Item)`.
- `Item{Label, Matches, Description, Icon, Leading, Git, Count, Strike, Muted, Tooltip, Actions, Menu, OnClick, OnDoubleClick, Drop}`.
- Selection, focus outline, hover, indent guides (on hover and focus), twistie, highlight ranges, git decoration, count and drop target come for free. Actions show on hover and on the focused row.
- `Spans(label, matches, color, highlight)` for match highlighting elsewhere.

### pane

- `ViewTitle(c, title, TitleOptions{Actions, ShowActions, Busy})`, `Header(c, title, &open, SectionOptions{Count, Actions, ShowActions, Busy, Border})`, `Section(c, title, &open, SectionOptions{}, body)`.
- `Stack(c, &StackState{}, []Pane{...})`: stacked sections with sashes; open sections share the height.
- `Toolbar(c, label, children)`, `Separator(c, vertical)`.

### chrome

- `NavButton(c, icon, commandID, history)`: back and forward. Disabled when the command is; right-click or long-press opens the history menu.
- `CommandCenter(c, text, commandID)`, `ActivityBar(c, label, &selected, []Activity{...})`.
- `TabStrip(c, &selected, []Tab{...}, TabStripOptions{Label, OnClose, Menu, Actions})`: dirty dot, close on hover and on the active tab, preview italics, pinned, git color.
- `Breadcrumbs(c, label, []Crumb{{Label, Icon, Menu}})`, `StatusItem(c, text, StatusOptions{Icon, Tooltip, Tone, Command, Menu})`.

### badge

- `Count(c, n, CountOptions{Accent, Small, Label})` (caps at 999+), `Decoration(c, gitStatus)`, `Tag(c, text, TagOptions{Tone, Icon, Tooltip})`, `Avatar(c, name, token.AvatarSize)`, `Keys(c, "⌘K ⌘S")`.

### feedback

- `ProgressBar(c, label)` (2 px, indeterminate), `Progress(c, label, value)`, `Spinner(c, label)`, `Loading(c, text)`.
- `Empty(c, EmptyOptions{Icon, Title, Message, Action, OnAction, Link, OnLink})`, `Error(c, title, err, retry)`.
- `Notice(c, NoticeOptions{Tone, Text, Action, OnAction, OnDismiss})`, `Toast(c, ToastOptions{Tone, Title, Message, Action, OnAction})`.

### quickinput

- `View(c, label, &State{}, []Item{...}, Options{Placeholder, Busy, Empty}) Result{Accepted, Closed}`: 600 px under the title bar; Up, Down, Enter, Escape and click outside.

### design (root)

- `Apply(c, Settings{Appearance, SystemAccent}) *token.Palette`: sets the theme each frame.
- `Text(c, s, style)`, `Stack(c, gap, body)`, `Inline(c, gap, body)`, `Spacer(c)`, `ViewBody(c, body)`, `Page(c, title, body)`, `Form(c, label, body)`, `Setting(c, title, description, control)`.
- `Gallery(c)`: every component in dark and light. `DESIGN_SHOTS=<dir> go test ./internal/design -run 'Gallery|QuickInput'` writes PNGs.

## Redlines and optical alignment

`internal/design/redline` reviews a screenshot the way a designer does with red lines. `redline.Analyze` takes the PNG, the accessibility boxes and the scale. It returns findings, an annotated PNG and a text report.

The annotated PNG shows:

- Red outlines on every box, and red gap dimensions between neighbours.
- Pink padding dimensions inside containers.
- Cyan guides on edges that 3 or more boxes share.
- Purple boxes on measured ink (icons and text cap bands), and yellow `#n` markers on findings.

| Rule | Flags | Threshold |
| --- | --- | --- |
| `align` | Edges or centres of related boxes that almost line up | 1 device px to 3 px |
| `gap` | A gap in a row or column of same-role boxes that differs from the others | 1 device px to 4 px off the median |
| `padding` | Container padding off the 4 px grid (a 1 px border is allowed) | Any |
| `optical` | Icon ink centre off the box centre, or a row icon off the row centre | More than 0.5 px, and more than 1 device px |
| `text-band` | Cap band (cap top to baseline) off the centre of a box with its own background | More than 0.5 px, and more than 1 device px |
| `crisp` | A fill or border edge on a part pixel that blurs | Edge off the device grid with a blended pixel |
| `icon-set` | Monochrome icons in one group with a different optical size or stroke weight | Size ×/÷ 1.5, stroke ±0.5 px |
| `contrast` | Real-pixel contrast of text and icons against their background, also hovered, pressed and selected | 4.5:1 text, 3:1 icons |

Each finding in the report names the element (role, label, bounds), the area of the view it is in, the rule, the measured value, the expected value, a fix with a token, the likely component, an allow key and a crop PNG.

### Optical centering rules

- Text: `token.TextStyle.Apply` moves text up by `token.TextNudgeY` (-0.5 px) with `token.NudgeText`. Use `TextStyle.Boxed` for text that draws its own background or border, and put a nudged text inside a `ui.Row` instead.
- Text inside a fill (badges, keycaps, chips): build the fill as a `ui.Row` with `AlignItems(ui.Center)` and `Justify(ui.Center)`, with the text as a child. Never set `Height` on a `ui.Text` to centre it. MyGo draws the text at the top of the box.
- Fills next to text (chips, keycaps): `token.TextStyle.WholeText` gives the text a whole-px width so that the next fill starts on a whole pixel.
- Bars: children of a bar with a top border use `FillHeight`, not the bar height token, so that they do not sit on a half pixel.

### Gates

Fix findings in `internal/design` with tokens. Do not use raw numbers. Check every UI change in dark and light:

- Components: `go test ./internal/design -run TestGalleryRedlines` renders the gallery headless at scale 2 in dark, light, dark high contrast and light high contrast. It also checks hover, pressed and keyboard focus for each control. Set `DESIGN_SHOTS=<dir>` to write `<view>.redline.png`, `<view>.redline.txt` and the crops. Set `DESIGN_REDLINE_DETAIL=1` to add every measurement to the report.
- Views: `EDITOR_E2E_ARTIFACTS=<dir> go test -tags e2e -run TestViewRedlines ./e2e/` opens each view of the demo project in dark and light. It also checks hover states. Each view writes `<view>-<state>-<theme>.png` and the redline files. The app captures at scale 1, so the optical threshold there is 1 px.

Both gates fail on any finding that is not in the allowlist. An allowlist entry is `<view prefix|*> <allow key prefix> -- <reason>`:

- Components: `internal/design/testdata/redline_allowlist.txt`
- Views: `e2e/testdata/redline_allowlist.txt`

Give each entry a reason that names the owner package and the px values. The allowlists only shrink. A stale entry fails the gate. `DESIGN_REDLINE_UPDATE=1` or `EDITOR_REDLINE_UPDATE=1` removes stale entries.

### Screenshot rule

Do not read screenshots to find problems. The text report has the numbers. Read at most one cropped image for each view, and only at the end, after all text checks pass. Read the per-finding crops only when a number is not enough to decide.

## Do and do not

| Do | Do not |
| --- | --- |
| `action.IconButton(c, icons.Refresh, "Refresh", action.IconOptions{Keys: "⌘R"})` | `ui.ButtonBase(c).Size(22, 22).Background(t.SurfaceHover)` |
| `ui.Row(c).Gap(token.GapInline.Px())` in design code, `design.Inline(c, token.GapInline, ...)` in features | `ui.Row(c).Gap(4)` |
| `list.Item{Git: token.GitModified}` | `TextColor(ui.Hex("#E2C08D"))` |
| `feedback.Empty(...)` with an action | A gray "Nothing here" text |
| `badge.Tag(c, "bug", badge.TagOptions{Tone: token.ToneError})` | Custom pills with alpha math |
| `design.Text(c, "Changes", token.TextTitle)` | `strings.ToUpper` and `FontSize(11)` |
| `c.SetMenuPresentation(ui.PopoverMenus)` for menus drawn in the theme | Mixed native and drawn menus in one view |

## Migration checklist

Replace each hand-coded pattern with the component, then shrink `internal/design/testdata/rules_allowlist.txt` with `DESIGN_RULES_UPDATE=1 go test ./internal/design -run TestDesignRules`. The allowlist only shrinks.

| Pattern | Replace with |
| --- | --- |
| `iconButton`, `pressable` (workspace/chrome.go, gitui/widgets.go, prui/issueui widgets) | `action.IconButton`, `action.MenuIconButton`, `action.CommandIconButton` |
| `textButton`, `ui.Button` | `action.Button`, `action.SplitButton` |
| `ui.TextInput`, `ui.TextArea`, search boxes with toggles | `field.TextInput`, `field.TextArea`, `field.SearchInput` with `field.Toggle` |
| `ui.Checkbox`, `ui.Switch`, `ui.Select` | `action.Checkbox`, `action.Switch`, `action.Select` |
| `sectionTitle`, sidebar section headers | `pane.Header`, `pane.Section`, `pane.Stack` |
| View title rows | `pane.ViewTitle` |
| `emptyState`, `loadingState`, `errorState` | `feedback.Empty`, `feedback.Loading`, `feedback.Error` |
| `badge`, count pills | `badge.Count`, `badge.Tag` |
| Palette rows and `palette.Highlight` | `quickinput.View`, `list.Spans` |
| File tree guides, twisties and selection override | `list.Tree` |
| Status bar items | `chrome.StatusItem` |
| Tab bar | `chrome.TabStrip` |
| Activity bar | `chrome.ActivityBar` |
| Title bar, command center, back and forward | `chrome.CommandCenter`, `chrome.NavButton`; `TitleBarHeight` goes from 52 to `token.TitleBarHeight` (35) |
| Breadcrumbs | `chrome.Breadcrumbs` |
| `c.Theme()` colors, `ui.RGB`/`ui.Hex` | `token.Of(c)` roles |
| Named numeric constants (`rowHeight = 30`, `panelRadius = 10`) | Semantic tokens; delete the constants |

Files with violations today (count in parentheses):

- internal/app: welcome.go (17)
- internal/workspace: activitybar.go (23), filetree/view.go (21), tabbar.go (20), status.go (18), toolwindows.go (17), watermark.go (15), titlebar.go (14), sidebar.go (13), breadcrumbs.go (12), github.go (9), chrome.go (7), editorgroup.go (7), prompt.go (6), insights.go (3), viewers.go (3), brackets.go (1), view.go (1)
- internal/gitui: commitview.go (42), widgets.go (38), branches.go (31), logview.go (29), push.go (19), worktrees.go (19), merge.go (17), conflicts.go (16), patchtab.go (16), statusbar.go (16), logdetails.go (15), stash.go (14), diffview.go (12), rebase.go (12), changes.go (11), remotes.go (11), submodules.go (11), linechange.go (10), shelf.go (10), namedialogs.go (9)
- internal/prui: detailview.go (95), diff.go (43), create.go (36), widgets.go (34), sideview.go (33), dialog.go (23), stateview.go (14), notifications.go (12), status.go (7), logs.go (5)
- internal/issueui: detailview.go (57), widgets.go (36), side.go (20), create.go (19), stateview.go (14), working.go (7), format.go (3)
- internal/search/view: ui.go (61), uirows.go (30), widgets.go (30), uirows2.go (12)
- internal/viewer: chrome.go (29), hex.go (16), media.go (16), image.go (10), pdf.go (5), markdown.go (1)
- internal/history: page.go (33)
- internal/insights: list.go (28)
- internal/palette: palette.go (23)
- internal/prefs: keys.go (34), settings.go (30), common.go (7)
- internal/largefile: findbar.go (9), view.go (6)

## Known gaps

- MyGo has no keyboard-only focus state on list rows, so row actions show on any focus of the list, not only keyboard focus.
- The tester cannot force hover, so the gallery shows hover through tests rather than screenshots.
- A long-press history menu uses a hidden anchor with `PopoverMenuButtonOptions.Open`; MyGo has no long-press menu on a button.
- Native menus ignore the palette; use popover menus for themed menus.

## Owner decisions (2026-11-26)

| Topic | Decision |
|---|---|
| Appearance | Follow the system, with a setting to force Light or Dark (`workbench.colorTheme`: System, Light, Dark). |
| Accent | The macOS system accent color. |
| Surfaces | Opaque theme colors for the title bar, sidebar, panel and status bar. No vibrancy. |
| Density | VS Code default: 22 px rows, 35 px tabs, 22 px status bar, 13 px UI text. |
| Editor font | JetBrains Mono 13 px, bundled with its OFL license. |
| Tabs | Xcode-like: rounded, borderless tabs with a soft fill for the active tab. |
| Status bar | Neutral. Color only for warnings and conflicts. |
| Title bar | 36 px. The command center shows the project name. The window title is "file - project". |
| Motion | None. Everything is instant (owner, 2026-11-26: "everything should just be snappy"). |
| Panel | Bottom by default. A command moves it to the right and remembers it. |
| Icons | Hugeicons 1.5 px stroke at 16 px. The selected view icon uses the filled variant. |
| Problems and TODO | Bottom panel tabs, with counts in the status bar. |
| Preview tabs | On for single clicks. |
| Open Editors | Shown in the Explorer, collapsed by default. |
| Empty editor group | App logo and the shortcut list. |
| Minimap | Off. |
