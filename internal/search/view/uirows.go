package searchview

import (
	"errors"
	"strconv"
	"strings"

	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/badge"
	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/list"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/ZerGo0/editor/internal/search/everywhere"
	"github.com/ZerGo0/editor/internal/search/symbols"
	"github.com/egoist/mygo/ui"
)

func (v *View) resultList(c *ui.Context) {
	up := c.KeyDown(ui.KeyUp) && c.Modifiers() == 0
	if up && !v.upDown && v.list.Handle.Focused(c) && v.selected >= 0 && v.selected <= v.firstSelectable() {
		v.field.Focus()
	}
	v.upDown = up
	opts := list.ViewOptions{Label: labelResults, Tree: v.hasBranches(), Empty: v.emptyState}
	el := list.View(c.Key("results"), &v.list, len(v.rows), opts, func(i int) list.Item { return v.item(c, i) }).Role(ui.RoleTree)
	if v.scrollTo {
		v.scrollTo = false
		v.list.ScrollIntoView(v.selected)
	}
	if el.Changed() {
		v.userMoved = true
		v.captureSelection()
		if r, ok := v.rowAt(v.selected); ok && r.kind != rowFile && r.kind != rowGroup && r.kind != rowFolder {
			v.previewRow(v.selected)
		}
	}
	if el.Submitted() {
		v.activateRow(v.selected, OpenOptions{Focus: true})
	}
	v.listKeys(c)
}

func (v *View) listKeys(c *ui.Context) {
	if !v.list.Handle.Focused(c) {
		return
	}
	r, ok := v.rowAt(v.selected)
	if !ok {
		return
	}
	switch {
	case c.Shortcut(ui.Cmd, ui.KeyEnter):
		v.activateRow(v.selected, OpenOptions{Side: true, Focus: true})
	case c.Shortcut(0, ui.KeySpace):
		v.lastPreview = ""
		v.previewRow(v.selected)
	case r.branch && c.Shortcut(0, ui.KeyRight) && !r.open:
		v.toggle(r.key)
	case r.branch && c.Shortcut(0, ui.KeyLeft) && r.open:
		v.toggle(r.key)
	case r.canDismiss() && (c.Shortcut(0, ui.KeyDelete) || c.Shortcut(0, ui.KeyBackspace)):
		v.dismiss(r)
	}
}

func (v *View) emptyState(c *ui.Context) {
	switch {
	case v.busy() || v.text.searching:
		feedback.Loading(c, "Searching…")
	case v.scope == ScopeText && v.text.err != "":
		feedback.Error(c, "The pattern is not valid", errors.New(v.text.err), func() { v.searchSoon(0) })
	case v.scope == ScopeText && strings.TrimSpace(v.st.Query) == "":
		feedback.Empty(c, feedback.EmptyOptions{Icon: icons.Search, Title: "Type to search the text of your files", Message: "Use .* for a regular expression. Shift+Enter adds a line."})
	case v.scope == ScopeText:
		feedback.Empty(c, feedback.EmptyOptions{Icon: icons.Search, Title: "No results for " + strconv.Quote(firstLine(strings.TrimSpace(v.st.Query))), Message: "Check the files to include and exclude, or the match options."})
	case v.scope == ScopeAll:
		feedback.Empty(c, feedback.EmptyOptions{Icon: icons.Search, Title: "Search files, symbols, text and actions", Message: "Start with # for symbols, @ for this file, > for actions, / for text or : for a line."})
	default:
		feedback.Empty(c, feedback.EmptyOptions{Icon: icons.Search, Title: "Type to search"})
	}
}

func (v *View) item(c *ui.Context, i int) list.Item {
	r, ok := v.rowAt(i)
	if !ok {
		return list.Item{}
	}
	it := list.Item{Depth: r.depth, Menu: func(m *ui.Menu) { v.rowMenu(c, m, r) }}
	switch r.kind {
	case rowHeader:
		return v.headerItem(r)
	case rowFolder:
		it.Leading, it.Label, it.Branch, it.Open, it.Count = folderLeading(r.title, r.open), r.title, true, r.open, r.count
		it.OnClick = func() { v.toggle(r.key) }
		it.Actions = v.rowActions(r)
	case rowFile, rowGroup:
		it.Leading, it.Label, it.Strong, it.Branch, it.Open, it.Count = fileLeading(r.title), r.title, true, true, r.open, r.count
		if r.detail != "" && (r.kind == rowGroup || v.st.ListView) {
			it.Description = r.detail
		}
		it.OnClick = func() { v.toggle(r.key) }
		if r.kind == rowFile {
			it.OnDoubleClick = func() { v.activateRow(i, OpenOptions{Focus: true}) }
		}
		it.Actions = v.rowActions(r)
	case rowMatch:
		v.matchItem(c, &it, r, i)
	case rowResult:
		v.resultItem(&it, r, i)
	case rowRecentQuery:
		it.Icon, it.Label = icons.History, firstLine(r.query)
		it.OnClick = func() { v.useQuery(r.query) }
		it.Menu = nil
	case rowGoto:
		it.Label = r.label
		if path := v.relPath(v.h.ActivePath()); path != "" {
			it.Description = "in " + path
		}
		it.Menu = nil
	case rowMessage:
		it.Label, it.Muted, it.Menu = r.title, true, nil
	}
	return it
}

func (v *View) headerItem(r row) list.Item {
	it := list.Item{Label: r.title, Strong: true, Count: r.count}
	if r.total > allSectionLimit && v.scope == ScopeAll {
		it.Trailing = func(c *ui.Context) {
			if action.Button(c, "Show all "+formatCount(r.total), action.ButtonOptions{Variant: action.Ghost, Small: true, ID: "all-" + r.title}).Clicked() {
				v.switchScope(r.scope)
			}
		}
	}
	if v.scope == ScopeAll {
		it.OnClick = func() {
			if r.scope != v.scope {
				v.switchScope(r.scope)
			}
		}
	}
	return it
}

func (v *View) matchItem(c *ui.Context, it *list.Item, r row, i int) {
	if r.match >= len(r.file.matches) {
		return
	}
	pal := token.Of(c)
	m := r.file.matches[r.match]
	replacing := v.scope == ScopeText && v.st.ShowReplace && v.text.matcher != nil
	replacement := ""
	if replacing {
		replacement = v.replacement().Expand(m.Matched, m.Groups)
	}
	it.Leading = func(c *ui.Context) {
		ui.Text(c, strconv.Itoa(m.Start.Line+1)).FontSize(token.TextCaption.Px()).SingleLine().TextColor(pal.Text.Secondary).
			Width(token.LineNumberWidth.Px()).TextAlign(ui.End)
	}
	it.Rich = func(s list.Style) []ui.Span { return matchSpans(pal, s, m, replacement, replacing) }
	it.Description = r.detail
	it.OnDoubleClick = func() { v.activateRow(i, OpenOptions{Focus: true}) }
	it.Actions = v.rowActions(r)
}

func (v *View) rowActions(r row) func(c *ui.Context) {
	if v.scope != ScopeText {
		return nil
	}
	replace := v.st.ShowReplace && (r.kind == rowMatch || r.kind == rowFile)
	dismiss := r.canDismiss()
	if !replace && !dismiss {
		return nil
	}
	return func(c *ui.Context) {
		if replace {
			if action.IconButton(c, icons.Replace, "Replace", action.IconOptions{Disabled: !v.canReplace(), ID: "replace-" + r.key}).Clicked() {
				if r.kind == rowMatch {
					v.replaceMatch(r.file, r.match)
				} else {
					v.replaceFile(r.file)
				}
			}
		}
		if dismiss {
			if action.IconButton(c, icons.Close, "Dismiss", action.IconOptions{ID: "dismiss-" + r.key}).Clicked() {
				v.dismiss(r)
			}
		}
	}
}

func (v *View) resultItem(it *list.Item, r row, i int) {
	res := r.res
	it.Label, it.Matches = res.Label, res.Positions
	switch res.Kind {
	case everywhere.KindFile:
		it.Leading = fileLeading(res.Label)
		it.Description = res.Detail
	case everywhere.KindSymbol:
		it.Leading = symbolLeading(res)
		it.Description = symbolDetail(res)
	case everywhere.KindGitRef:
		it.Icon, it.Description = icons.GitBranch, res.Detail
	case everywhere.KindGitCommit:
		it.Icon, it.Description = icons.Commit, res.Detail
	}
	it.Trailing = func(c *ui.Context) {
		if res.Kind == everywhere.KindAction && res.Keys != "" {
			badge.Keys(c, res.Keys)
		}
		if res.Open {
			ui.Icon(c, icons.Dot).Size(token.DirtyDotSize.Px(), token.DirtyDotSize.Px()).TextColor(token.Of(c).Text.Secondary).Tooltip("Open in an editor").Shrink(0)
		}
	}
	it.OnDoubleClick = func() { v.activateRow(i, OpenOptions{Focus: true}) }
}

func symbolDetail(res everywhere.Result) string {
	text := res.Detail + ":" + itoa(res.Line+1)
	if res.Container != "" {
		text = res.Container + "  " + text
	}
	return text
}

func symbolLeading(res everywhere.Result) func(c *ui.Context) {
	return func(c *ui.Context) {
		badge.Tag(c, kindLetters[res.Symbol], badge.TagOptions{Tone: kindTone(res.Symbol), Tooltip: res.Symbol.String()})
	}
}

func kindTone(k symbols.Kind) token.Tone {
	switch k {
	case symbols.Function, symbols.Method:
		return token.ToneInfo
	case symbols.Class, symbols.Struct, symbols.Interface, symbols.Enum, symbols.Type:
		return token.ToneWarning
	case symbols.Const, symbols.Var, symbols.Field:
		return token.ToneSuccess
	}
	return token.ToneNeutral
}
