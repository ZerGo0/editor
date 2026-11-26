package searchview

import (
	"strings"

	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

const (
	labelFilesInclude = "files to include"
	labelFilesExclude = "files to exclude"
	labelSearch       = "Search"
	labelReplace      = "Replace"
	labelResults      = "Search results"
	truncatedNotice   = "The result set only contains a subset of all matches. Be more specific in your search to narrow down the results."
)

var scopeKeys = [scopeCount]ui.Key{ui.Key1, ui.Key2, ui.Key3, ui.Key4, ui.Key5, ui.Key6}

func (v *View) View(c *ui.Context) {
	v.tick(c)
	if v.rowsDirty {
		v.rebuild()
	}
	ui.Column(c).Grow(1).FillWidth().MinHeight(0).Children(func() {
		v.chips(c)
		v.fieldBlock(c)
		v.statusRow(c)
		v.resultList(c)
	})
	v.windowKeys(c)
}

func (v *View) rebuild() {
	v.rowsDirty = false
	v.rows = v.buildRows()
	idx := v.indexOfKey(v.selectedKey)
	if v.scope != ScopeText && (idx < 0 || !v.userMoved) {
		idx = v.firstSelectable()
	}
	v.choose(idx)
	if v.openPending && !v.busy() {
		v.openPending = false
		v.activateRow(v.selected, OpenOptions{Focus: true})
	}
}

func (v *View) indexOfKey(key string) int {
	if key == "" {
		return -1
	}
	for i, r := range v.rows {
		if r.key == key {
			return i
		}
	}
	return -1
}

func (v *View) firstSelectable() int {
	for i, r := range v.rows {
		if r.kind != rowHeader && r.kind != rowMessage {
			return i
		}
	}
	return -1
}

func (v *View) choose(i int) {
	v.selected = i
	v.list.Choose(i)
	v.captureSelection()
}

func (v *View) captureSelection() {
	if r, ok := v.rowAt(v.selected); ok {
		v.selectedKey = r.key
		return
	}
	v.selectedKey = ""
}

func (v *View) chips(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).FillWidth().Padding(0, sidePad, 6, sidePad).Gap(4).Wrap().Role(ui.RoleTabList).Label("Search scope").Children(func() {
		for s := ScopeAll; s < scopeCount; s++ {
			active := s == v.scope
			b := ui.ButtonBase(c.Key("scope-"+s.String())).Height(buttonSize-2).Padding(0, 8).Radius(chipRadius).Gap(4).AlignItems(ui.Center).
				Role(ui.RoleTab).AccessSelected(active).Label(s.String()).Tooltip(s.String() + " (" + scopeShortcut(s) + ")")
			switch {
			case active:
				b.Background(t.Accent.Alpha(tintAlpha)).BorderWidth(1).BorderColor(t.Accent)
			case b.Hovered():
				b.Background(t.SurfaceHover)
			}
			n := v.scopeCount(s)
			b.Children(func() {
				ui.Text(c, s.String()).FontSize(smallFont).SingleLine().TextColor(ifColor(active, t.Text, t.TextMuted))
				if n > 0 {
					ui.Text(c, formatCount(n)).FontSize(tinyFont).SingleLine().TextColor(t.TextMuted)
				}
			})
			if b.Clicked() {
				v.switchScope(s)
				v.field.Focus()
			}
		}
	})
}

func scopeShortcut(s Scope) string {
	return "⌘" + itoa(int(s)+1)
}

func (v *View) scopeCount(s Scope) int {
	if strings.TrimSpace(v.st.Query) == "" {
		return 0
	}
	if s != v.scope && v.scope != ScopeAll {
		return 0
	}
	switch s {
	case ScopeText:
		return v.text.total
	case ScopeAll:
		return 0
	}
	return v.sections[s].total
}

func (v *View) placeholder() string {
	switch v.scope {
	case ScopeFiles:
		return "Go to file (add :line to jump)"
	case ScopeSymbols:
		if v.editorOnly {
			return "Go to symbol in editor"
		}
		return "Go to symbol in workspace"
	case ScopeText:
		return "Search (Shift+Enter for a new line)"
	case ScopeActions:
		return "Run an action"
	case ScopeGit:
		return "Branches, tags and commits"
	}
	return "Search files, symbols, text and actions"
}

func (v *View) fieldBlock(c *ui.Context) {
	text := v.scope == ScopeText
	ui.Row(c).FillWidth().Padding(0, sidePad, 4, sidePad).Gap(2).AlignItems(ui.Start).Children(func() {
		if text {
			chev := icons.ChevronRight
			if v.st.ShowReplace {
				chev = icons.ChevronDown
			}
			if iconButton(c, "toggle-replace", "Toggle Replace", chev, false).Width(chevronWidth).Clicked() {
				v.st.ShowReplace = !v.st.ShowReplace
			}
		}
		v.searchBox(c)
	})
	if text && v.st.ShowReplace {
		v.replaceRow(c)
	}
	if text && v.st.ShowFilters {
		v.filterFields(c)
	}
	if text && v.text.err != "" {
		ui.Text(c, v.text.err).FontSize(tinyFont).TextColor(c.Theme().Danger).Padding(0, sidePad, 4, sidePad+chevronWidth+2).Role(ui.RoleAlert)
	}
}

func (v *View) boxBorder(c *ui.Context, handle *ui.Handle, invalid bool) ui.Color {
	t := c.Theme()
	switch {
	case invalid:
		return t.Danger
	case handle.Focused(c):
		return t.Accent
	}
	return t.Border
}

func (v *View) searchBox(c *ui.Context) {
	t := c.Theme()
	text := v.scope == ScopeText
	box := ui.Row(c.Key("search-box")).Grow(1).MinWidth(0).Background(t.Surface).Radius(fieldRadius).BorderWidth(1).
		BorderColor(v.boxBorder(c, &v.field, text && v.text.err != "")).Padding(1, 2, 1, 6).AlignItems(ui.Start).Role(ui.RoleSearch)
	box.Children(func() {
		in := ui.TextAreaBase(c, &v.st.Query).Bind(&v.field).Label(labelSearch).Placeholder(v.placeholder()).Lines(1, maxFieldLines).
			FontSize(bodyFont).Grow(1).MinWidth(0).Padding(3, 2, 3, 0).OnTextKey(v.fieldKey)
		if v.pendingNewline {
			v.pendingNewline = false
			from, to := in.TextSelection()
			in.ReplaceText(from, to, "\n")
		}
		if in.Changed() {
			v.queryEdited()
		}
		if !text {
			return
		}
		ui.Row(c).Gap(1).Shrink(0).Padding(1, 0).Children(func() {
			changed := flagToggle(c, &v.st.MatchCase, icons.CaseSensitive, "Match Case", "Match Case (⌥⌘C)")
			changed = flagToggle(c, &v.st.WholeWord, icons.WholeWord, "Match Whole Word", "Match Whole Word (⌥⌘W)") || changed
			changed = flagToggle(c, &v.st.Regex, icons.Regex, "Use Regular Expression", "Use Regular Expression (⌥⌘R)") || changed
			if changed {
				v.searchSoon(0)
			}
		})
	})
}

func (v *View) replaceRow(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).FillWidth().Padding(0, sidePad, 4, sidePad+chevronWidth+2).Gap(2).AlignItems(ui.Start).Children(func() {
		box := ui.Row(c.Key("replace-box")).Grow(1).MinWidth(0).Background(t.Surface).Radius(fieldRadius).BorderWidth(1).
			BorderColor(v.boxBorder(c, &v.replaceField, false)).Padding(1, 2, 1, 6).AlignItems(ui.Start)
		box.Children(func() {
			in := ui.TextAreaBase(c, &v.st.Replace).Bind(&v.replaceField).Label(labelReplace).Placeholder(labelReplace).Lines(1, maxFieldLines).
				FontSize(bodyFont).Grow(1).MinWidth(0).Padding(3, 2, 3, 0).OnTextKey(v.replaceKey)
			if v.pendingReplaceNewline {
				v.pendingReplaceNewline = false
				from, to := in.TextSelection()
				in.ReplaceText(from, to, "\n")
			}
			ui.Row(c).Shrink(0).Padding(1, 0).Children(func() {
				flagToggle(c, &v.st.PreserveCase, icons.PreserveCase, "Preserve Case", "Preserve Case (⌥⌘P)")
			})
		})
		if iconButton(c, "replace-all", "Replace All", icons.ReplaceAll, !v.canReplace()).Clicked() {
			v.ReplaceAll()
		}
	})
}

func (v *View) filterFields(c *ui.Context) {
	t := c.Theme()
	changed := false
	ui.Column(c).FillWidth().Padding(0, sidePad, 6, sidePad+chevronWidth+2).Gap(4).Children(func() {
		ui.Text(c, labelFilesInclude).FontSize(tinyFont).TextColor(t.TextMuted)
		changed = ui.TextInput(c.Key("include"), &v.st.Include).Bind(&v.includeField).Label(labelFilesInclude).Placeholder("e.g. src/**/*.go, ./internal").FontSize(smallFont).FillWidth().Changed() || changed
		changed = ui.Checkbox(c, &v.st.OnlyOpen, "Search only in open editors").Label("Search only in open editors").Changed() || changed
		ui.Text(c, labelFilesExclude).FontSize(tinyFont).TextColor(t.TextMuted)
		changed = ui.TextInput(c.Key("exclude"), &v.st.Exclude).Label(labelFilesExclude).Placeholder("e.g. **/*.test.go, dist").FontSize(smallFont).FillWidth().Changed() || changed
		use := !v.st.NoExcludes
		if ui.Checkbox(c, &use, "Use exclude settings and ignore files").Label("Use exclude settings and ignore files").Changed() {
			v.st.NoExcludes = !use
			changed = true
		}
	})
	if changed {
		v.searchSoon(textDelay)
	}
}

func (v *View) statusRow(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).FillWidth().MinHeight(buttonSize+4).Padding(0, sidePad, 0, sidePad+2).Gap(4).AlignItems(ui.Center).Children(func() {
		v.statusMessage(c)
		ui.Box(c).Grow(1)
		v.headerActions(c)
	})
	if v.scope == ScopeText && v.text.truncated && !v.text.searching {
		ui.Text(c, truncatedNotice).FontSize(tinyFont).TextColor(t.Warning).Padding(0, sidePad, 4, sidePad+2)
	}
}

func (v *View) statusMessage(c *ui.Context) {
	t := c.Theme()
	switch {
	case v.text.searching && v.scope != ScopeAll:
		ui.Spinner(c).Size(spinnerSize, spinnerSize).Label("Searching").Shrink(0)
		ui.Text(c, "Searching…").FontSize(smallFont).TextColor(t.TextMuted).SingleLine()
		if textButton(c, "cancel-search", "Cancel", false).Clicked() {
			v.CancelSearch()
		}
	case v.busy():
		ui.Spinner(c).Size(spinnerSize, spinnerSize).Label("Loading").Shrink(0)
	case v.scope == ScopeText && v.text.err == "" && strings.TrimSpace(v.st.Query) != "" && v.text.matcher != nil:
		ui.Text(c, v.statusSummary()).FontSize(smallFont).TextColor(t.TextMuted).SingleLine().Role(ui.RoleStatus)
	}
}

func (v *View) statusSummary() string {
	if v.text.total == 0 {
		return "No results found"
	}
	return v.summary()
}

func (v *View) headerActions(c *ui.Context) {
	hasQuery := strings.TrimSpace(v.st.Query) != ""
	if iconButton(c, "refresh", "Refresh", icons.Refresh, !hasQuery).Clicked() {
		v.Refresh()
	}
	if iconButton(c, "clear", "Clear Search Results", icons.ClearAll, !hasQuery && v.text.total == 0).Clicked() {
		v.ClearResults()
	}
	if v.hasBranches() {
		label, svg := "Collapse All", icons.CollapseAll
		if !v.anyExpanded() {
			label, svg = "Expand All", icons.ExpandAll
		}
		if iconButton(c, "collapse-all", label, svg, false).Clicked() {
			v.toggleAll()
		}
	}
	switch v.scope {
	case ScopeText:
		label, svg := "View as List", icons.ListView
		if v.st.ListView {
			label, svg = "View as Tree", icons.TreeView
		}
		if iconButton(c, "view-mode", label, svg, false).Clicked() {
			v.st.ListView = !v.st.ListView
			v.rowsDirty = true
		}
		if iconButton(c, "toggle-filters", "Toggle Search Details", icons.More, false).Clicked() {
			v.st.ShowFilters = !v.st.ShowFilters
		}
	case ScopeSymbols:
		label, svg := "Group by File", icons.TreeView
		if v.st.SymbolsTree {
			label, svg = "View as List", icons.ListView
		}
		if iconButton(c, "view-mode", label, svg, false).Clicked() {
			v.st.SymbolsTree = !v.st.SymbolsTree
			v.rowsDirty = true
		}
	}
}

func (v *View) queryEdited() {
	v.historyPos = -1
	v.userMoved = false
	if v.scope == ScopeAll {
		if r := routePrefix(v.st.Query); r.matched {
			v.setScope(r.scope)
			v.editorOnly = r.inEditor
			v.st.Query = r.query
		}
	}
	v.searchSoon(textDelay)
}

func (v *View) cycleScope(step int) {
	next := (int(v.scope) + step + int(scopeCount)) % int(scopeCount)
	v.switchScope(Scope(next))
}

const flagMods = ui.Alt | ui.Super

func (v *View) fieldKey(k ui.TextKey) bool {
	text := v.scope == ScopeText
	switch {
	case k.Key == ui.KeyEnter && k.Mods == 0:
		v.submit()
		return true
	case k.Key == ui.KeyEnter && k.Mods == ui.Shift:
		return !text
	case k.Key == ui.KeyEnter && k.Mods == ui.Cmd:
		if text {
			v.pendingNewline = true
			return true
		}
		v.activateRow(v.selected, OpenOptions{Side: true, Focus: true})
		return true
	case k.Key == ui.KeyTab && (k.Mods == 0 || k.Mods == ui.Shift):
		step := 1
		if k.Mods == ui.Shift {
			step = -1
		}
		v.cycleScope(step)
		return true
	case k.Key == ui.KeyEscape && k.Mods == 0:
		v.escape()
		return true
	case k.Key == ui.KeyUp && k.Mods == 0:
		return v.historyUp(k)
	case k.Key == ui.KeyDown && k.Mods == 0:
		return v.downFromField(k)
	case k.Key == ui.KeyDown && k.Mods == ui.Cmd:
		v.enterList()
		return true
	}
	return v.flagKey(k)
}

func (v *View) flagKey(k ui.TextKey) bool {
	if k.Mods == ui.Cmd {
		for s, key := range scopeKeys {
			if k.Key == key {
				v.switchScope(Scope(s))
				return true
			}
		}
	}
	if v.scope != ScopeText || k.Mods != flagMods {
		return false
	}
	switch k.Key {
	case ui.KeyC:
		v.st.MatchCase = !v.st.MatchCase
	case ui.KeyW:
		v.st.WholeWord = !v.st.WholeWord
	case ui.KeyR:
		v.st.Regex = !v.st.Regex
	case ui.KeyP:
		v.st.PreserveCase = !v.st.PreserveCase
	default:
		return false
	}
	v.searchSoon(0)
	return true
}

func (v *View) replaceKey(k ui.TextKey) bool {
	switch {
	case k.Key == ui.KeyEnter && k.Mods == 0:
		v.ReplaceAll()
		return true
	case k.Key == ui.KeyEnter && k.Mods == ui.Cmd:
		v.pendingReplaceNewline = true
		return true
	case k.Key == ui.KeyEscape && k.Mods == 0:
		v.field.Focus()
		return true
	}
	return false
}

func (v *View) submit() {
	if v.scope == ScopeText {
		v.searchSoon(0)
		return
	}
	if v.quickDue.IsZero() && !v.quickArm && !v.busy() {
		v.activateRow(v.selected, OpenOptions{Focus: true})
		return
	}
	v.openPending = true
}

func (v *View) escape() {
	if v.st.Query != "" {
		v.st.Query = ""
		v.cancelAll()
		v.text = textResults{}
		v.sections = [scopeCount]section{}
		v.selected, v.selectedKey = -1, ""
		v.historyPos = -1
		v.searchSoon(0)
		v.rowsDirty = true
		return
	}
	if v.env.FocusEditor != nil {
		v.env.FocusEditor()
	}
}

func (v *View) historyUp(k ui.TextKey) bool {
	if v.scope != ScopeText || strings.Contains(k.Text(), "\n") || len(v.st.History) == 0 {
		return v.scope != ScopeText
	}
	if v.historyPos < 0 {
		v.historyDraft = k.Text()
	}
	if v.historyPos+1 < len(v.st.History) {
		v.historyPos++
		k.SetText(v.st.History[v.historyPos])
		v.searchSoon(textDelay)
	}
	return true
}

func (v *View) downFromField(k ui.TextKey) bool {
	if v.scope == ScopeText && strings.Contains(k.Text(), "\n") {
		return false
	}
	if v.historyPos >= 0 {
		v.historyPos--
		if v.historyPos < 0 {
			k.SetText(v.historyDraft)
		} else {
			k.SetText(v.st.History[v.historyPos])
		}
		v.searchSoon(textDelay)
		return true
	}
	v.enterList()
	return true
}

func (v *View) enterList() {
	if v.selected < 0 {
		v.choose(v.firstSelectable())
	}
	v.list.Handle.Focus()
}

func (v *View) windowKeys(c *ui.Context) {
	inside := v.field.FocusWithin(c) || v.list.Handle.FocusWithin(c) || v.replaceField.FocusWithin(c)
	if !inside {
		return
	}
	if v.list.Handle.FocusWithin(c) && c.Shortcut(0, ui.KeyEscape) {
		v.escape()
	}
}
