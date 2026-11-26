package searchview

import (
	"strings"

	"github.com/ZerGo0/editor/internal/design"
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/field"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

const (
	flagCount         = 3
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
	ui.Row(c).FillWidth().Padding(0, token.PadRow.Px(), token.GapControl.Px(), token.PadRow.Px()).Gap(token.GapTight.Px()).Wrap().
		Role(ui.RoleTabList).Label("Search scope").Children(func() {
		for s := ScopeAll; s < scopeCount; s++ {
			count := ""
			if n := v.scopeCount(s); n > 0 {
				count = formatCount(n)
			}
			chip := action.Chip(c, s.String(), action.ChipOptions{Active: s == v.scope, Count: count, Keys: scopeShortcut(s), ID: "scope-" + s.String()})
			if chip.Clicked() {
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
		return "Go to file"
	case ScopeSymbols:
		if v.editorOnly {
			return "Go to symbol in editor"
		}
		return "Go to symbol"
	case ScopeText:
		return "Search"
	case ScopeActions:
		return "Run an action"
	case ScopeGit:
		return "Branches, tags and commits"
	}
	return "Search everywhere"
}

func (v *View) fieldInset() float32 {
	return token.PadRow.Px() + token.IconButtonSize.Px() + token.GapTight.Px()
}

func (v *View) fieldBlock(c *ui.Context) {
	text := v.scope == ScopeText
	ui.Row(c).FillWidth().Padding(0, token.PadRow.Px(), token.GapInline.Px(), token.PadRow.Px()).Gap(token.GapTight.Px()).AlignItems(ui.Start).Children(func() {
		if text {
			chevron := icons.ChevronRight
			if v.st.ShowReplace {
				chevron = icons.ChevronDown
			}
			if action.IconButton(c, chevron, "Toggle Replace", action.IconOptions{ID: "toggle-replace"}).Margin(token.GapTight.Px(), 0, 0, 0).Clicked() {
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
}

func (v *View) searchBox(c *ui.Context) {
	text := v.scope == ScopeText
	o := field.AreaOptions{Placeholder: v.placeholder(), MaxLines: maxFieldLines}
	flags := [flagCount]bool{v.st.MatchCase, v.st.WholeWord, v.st.Regex}
	if text {
		o.Error = v.text.err
		o.Toggles = []field.Toggle{
			{Icon: icons.CaseSensitive, Label: "Match Case", Keys: "⌥⌘C", On: &v.st.MatchCase},
			{Icon: icons.WholeWord, Label: "Match Whole Word", Keys: "⌥⌘W", On: &v.st.WholeWord},
			{Icon: icons.Regex, Label: "Use Regular Expression", Keys: "⌥⌘R", On: &v.st.Regex},
		}
	}
	ui.Column(c.Key("search-field")).Grow(1).MinWidth(0).Children(func() {
		in := field.TextArea(c, labelSearch, &v.st.Query, o).Bind(&v.field).OnTextKey(v.fieldKey)
		if v.pendingNewline {
			v.pendingNewline = false
			from, to := in.TextSelection()
			in.ReplaceText(from, to, "\n")
		}
		if in.Changed() {
			v.queryEdited()
		}
	})
	if text && flags != [flagCount]bool{v.st.MatchCase, v.st.WholeWord, v.st.Regex} {
		v.searchSoon(0)
	}
}

func (v *View) replaceRow(c *ui.Context) {
	ui.Row(c).FillWidth().Padding(0, token.PadRow.Px(), token.GapInline.Px(), v.fieldInset()).Gap(token.GapTight.Px()).AlignItems(ui.Start).Children(func() {
		ui.Column(c.Key("replace-field")).Grow(1).MinWidth(0).Children(func() {
			o := field.AreaOptions{Placeholder: labelReplace, MaxLines: maxFieldLines, Toggles: []field.Toggle{
				{Icon: icons.PreserveCase, Label: "Preserve Case", Keys: "⌥⌘P", On: &v.st.PreserveCase},
			}}
			in := field.TextArea(c, labelReplace, &v.st.Replace, o).Bind(&v.replaceField).OnTextKey(v.replaceKey)
			if v.pendingReplaceNewline {
				v.pendingReplaceNewline = false
				from, to := in.TextSelection()
				in.ReplaceText(from, to, "\n")
			}
		})
		if action.IconButton(c, icons.ReplaceAll, "Replace All", action.IconOptions{Disabled: !v.canReplace()}).Margin(token.GapTight.Px(), 0, 0, 0).Clicked() {
			v.ReplaceAll()
		}
	})
}

func (v *View) filterFields(c *ui.Context) {
	changed := false
	ui.Column(c).FillWidth().Padding(0, token.PadRow.Px(), token.GapControl.Px(), v.fieldInset()).Gap(token.GapInline.Px()).Children(func() {
		design.Text(c, labelFilesInclude, token.TextCaption)
		include := field.TextInput(c, labelFilesInclude, &v.st.Include, field.InputOptions{Placeholder: "e.g. src/**/*.go, ./internal", ID: "include"}).Bind(&v.includeField)
		changed = include.Changed() || changed
		changed = action.Checkbox(c, "Search only in open editors", &v.st.OnlyOpen, action.ChoiceOptions{}).Changed() || changed
		design.Text(c, labelFilesExclude, token.TextCaption)
		changed = field.TextInput(c, labelFilesExclude, &v.st.Exclude, field.InputOptions{Placeholder: "e.g. **/*.test.go, dist", ID: "exclude"}).Changed() || changed
		use := !v.st.NoExcludes
		if action.Checkbox(c, "Use exclude settings and ignore files", &use, action.ChoiceOptions{}).Changed() {
			v.st.NoExcludes = !use
			changed = true
		}
	})
	if changed {
		v.searchSoon(textDelay)
	}
}

func (v *View) statusRow(c *ui.Context) {
	ui.Row(c).FillWidth().MinHeight(token.ToolbarHeight.Px()).Padding(0, token.PadRow.Px(), 0, token.PadView.Px()).Gap(token.GapInline.Px()).AlignItems(ui.Center).Children(func() {
		v.statusMessage(c)
		design.Spacer(c)
		v.headerActions(c)
	})
	if v.scope == ScopeText && v.text.truncated && !v.text.searching {
		ui.Column(c).FillWidth().Padding(0, token.PadRow.Px(), token.GapInline.Px(), token.PadRow.Px()).Children(func() {
			feedback.Notice(c, feedback.NoticeOptions{Tone: token.ToneWarning, Text: truncatedNotice})
		})
	}
}

func (v *View) statusMessage(c *ui.Context) {
	switch {
	case v.text.searching && v.scope != ScopeAll:
		feedback.Spinner(c, "Searching").Shrink(0)
		design.Text(c, "Searching…", token.TextLabel).SingleLine()
		if action.Button(c, "Cancel", action.ButtonOptions{Variant: action.Ghost, Small: true, ID: "cancel-search"}).Clicked() {
			v.CancelSearch()
		}
	case v.busy():
		feedback.Spinner(c, "Loading").Shrink(0)
	case v.scope == ScopeText && v.text.err == "" && strings.TrimSpace(v.st.Query) != "" && v.text.matcher != nil:
		design.Text(c, v.statusSummary(), token.TextLabel).SingleLine().Role(ui.RoleStatus)
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
	if action.IconButton(c, icons.Refresh, "Refresh", action.IconOptions{Disabled: !hasQuery, ID: "refresh"}).Clicked() {
		v.Refresh()
	}
	if action.IconButton(c, icons.ClearAll, "Clear Search Results", action.IconOptions{Disabled: !hasQuery && v.text.total == 0, ID: "clear"}).Clicked() {
		v.ClearResults()
	}
	if v.hasBranches() {
		label, svg := "Collapse All", icons.CollapseAll
		if !v.anyExpanded() {
			label, svg = "Expand All", icons.ExpandAll
		}
		if action.IconButton(c, svg, label, action.IconOptions{ID: "collapse-all"}).Clicked() {
			v.toggleAll()
		}
	}
	switch v.scope {
	case ScopeText:
		label, svg := "View as List", icons.ListView
		if v.st.ListView {
			label, svg = "View as Tree", icons.TreeView
		}
		if action.IconButton(c, svg, label, action.IconOptions{ID: "view-mode"}).Clicked() {
			v.st.ListView = !v.st.ListView
			v.rowsDirty = true
		}
		if action.IconButton(c, icons.More, "Toggle Search Details", action.IconOptions{Active: v.st.ShowFilters, ID: "toggle-filters"}).Clicked() {
			v.st.ShowFilters = !v.st.ShowFilters
		}
	case ScopeSymbols:
		label, svg := "Group by File", icons.TreeView
		if v.st.SymbolsTree {
			label, svg = "View as List", icons.ListView
		}
		if action.IconButton(c, svg, label, action.IconOptions{ID: "view-mode"}).Clicked() {
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
