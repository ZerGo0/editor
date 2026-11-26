package searchview

import (
	"strconv"
	"strings"

	"github.com/ZerGo0/editor/internal/icons"
	"github.com/ZerGo0/editor/internal/search/everywhere"
	"github.com/egoist/mygo/ui"
)

func (v *View) resultList(c *ui.Context) {
	up := c.KeyDown(ui.KeyUp) && c.Modifiers() == 0
	if up && !v.upDown && v.list.Handle.Focused(c) && v.selected >= 0 && v.selected <= v.firstSelectable() {
		v.field.Focus()
	}
	v.upDown = up
	list := ui.List(c.Key("results"), &v.list, len(v.rows)).Grow(1).FillWidth().MinHeight(0).Label(labelResults).Role(ui.RoleTree)
	list.Rows(func(lr ui.ListRow) { v.rowView(lr) })
	list.Children(func() {
		if len(v.rows) == 0 {
			v.emptyState(c)
		}
	})
	if v.scrollTo {
		v.scrollTo = false
		v.list.ScrollIntoView(v.selected)
	}
	if list.Changed() {
		v.userMoved = true
		v.captureSelection()
		if r, ok := v.rowAt(v.selected); ok && r.kind != rowFile && r.kind != rowGroup && r.kind != rowFolder {
			v.previewRow(v.selected)
		}
	}
	if list.Submitted() {
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
	t := c.Theme()
	switch {
	case v.busy() || v.text.searching:
		ui.Row(c).FillWidth().Center().Gap(stateGap).Padding(statePadding).Children(func() {
			ui.Spinner(c).Size(spinnerSize, spinnerSize).Label("Searching")
			ui.Text(c, "Searching…").FontSize(smallFont).TextColor(t.TextMuted)
		})
	case v.scope == ScopeText && v.text.err != "":
		ui.Column(c).FillWidth().Center().Gap(stateGap).Padding(statePadding).Children(func() {
			ui.Text(c, "The pattern is not valid").FontSize(smallFont).TextColor(t.Danger).TextAlign(ui.Center)
			if ui.Button(c, "Retry").Clicked() {
				v.searchSoon(0)
			}
		})
	case v.scope == ScopeText && strings.TrimSpace(v.st.Query) == "":
		stateText(c, "Type to search the text of your files", "Use .* for a regular expression. Shift+Enter adds a line.")
	case v.scope == ScopeText:
		stateText(c, "No results for "+strconv.Quote(firstLine(strings.TrimSpace(v.st.Query))), "Check the files to include and exclude, or the match options.")
	case v.scope == ScopeAll:
		stateText(c, "Search files, symbols, text and actions", "Start with # for symbols, @ for this file, > for actions, / for text or : for a line.")
	default:
		stateText(c, "Type to search", "")
	}
}

func (v *View) rowView(lr ui.ListRow) {
	r, ok := v.rowAt(lr.Index)
	if !ok {
		return
	}
	switch r.kind {
	case rowHeader:
		v.headerView(lr, r)
	case rowFolder, rowFile, rowGroup:
		v.branchView(lr, r)
	case rowMatch:
		v.matchView(lr, r)
	case rowResult:
		v.resultView(lr, r)
	case rowRecentQuery:
		v.recentView(lr, r)
	case rowGoto:
		v.gotoView(lr, r)
	case rowMessage:
		mutedText(lr.Context, r.title, false).Padding(6, sidePad)
	}
}

func (v *View) box(lr ui.ListRow, r row) ui.Element {
	return ui.Row(lr.Context).Height(rowHeight).FillWidth().Padding(0, sidePad, 0, sidePad+float32(r.depth)*indentStep).Gap(5).AlignItems(ui.Center)
}

func (v *View) headerView(lr ui.ListRow, r row) {
	c := lr.Context
	t := c.Theme()
	bg := t.Background
	if c.Vibrancy() {
		bg = bg.Alpha(headerAlpha)
	}
	el := ui.Row(c).Height(rowHeight).FillWidth().Padding(0, sidePad).Gap(6).AlignItems(ui.Center).Background(bg)
	el.Children(func() {
		ui.Text(c, strings.ToUpper(r.title)).FontSize(tinyFont).Bold().TextColor(t.TextMuted).SingleLine()
		if r.count > 0 {
			countBadge(c, r.count, false)
		}
		ui.Box(c).Grow(1)
		if r.total > allSectionLimit && v.scope == ScopeAll {
			if textButton(c, "all-"+r.title, "Show all "+formatCount(r.total), false).Clicked() {
				v.switchScope(r.scope)
			}
		}
	})
	if el.RowClicked() && r.scope != v.scope && v.scope == ScopeAll {
		v.switchScope(r.scope)
	}
}

func (v *View) branchView(lr ui.ListRow, r row) {
	c := lr.Context
	sel := lr.Selected()
	el := v.box(lr, r).Role(ui.RoleTreeItem).Level(r.depth + 1).Expanded(r.open)
	el.Children(func() {
		chevron(c, r.open, sel)
		switch r.kind {
		case rowFolder:
			folderIcon(c, r.title, r.open)
			ui.Text(c, r.title).FontSize(bodyFont).SingleLine().Shrink(1).MinWidth(0)
		default:
			fileIcon(c, r.title)
			ui.Text(c, r.title).FontSize(bodyFont).SingleLine().Bold().Shrink(0)
			if r.detail != "" && (r.kind == rowGroup || v.st.ListView) {
				mutedText(c, r.detail, sel).SingleLine().TruncateStart().Shrink(1).MinWidth(0)
			}
		}
		ui.Box(c).Grow(1)
		if lr.Hovered() || sel {
			v.rowActions(c, r, sel)
		}
		if r.count > 0 {
			countBadge(c, r.count, sel)
		}
	})
	if el.RowClicked() {
		v.toggle(r.key)
	}
	if r.kind == rowFile && el.RowDoubleClicked() {
		v.activateRow(lr.Index, OpenOptions{Focus: true})
	}
	el.ContextMenu(func(m *ui.Menu) { v.rowMenu(c, m, r) })
}

func (v *View) matchView(lr ui.ListRow, r row) {
	c := lr.Context
	t := c.Theme()
	sel := lr.Selected()
	if r.match >= len(r.file.matches) {
		return
	}
	m := r.file.matches[r.match]
	replacing := v.scope == ScopeText && v.st.ShowReplace && v.text.matcher != nil
	replacement := ""
	if replacing {
		replacement = v.replacement().Expand(m.Matched, m.Groups)
	}
	el := v.box(lr, r).Role(ui.RoleTreeItem).Level(r.depth + 1)
	el.Children(func() {
		ui.Text(c, strconv.Itoa(m.Start.Line+1)).FontSize(tinyFont).SingleLine().TextColor(mutedColor(t, sel)).Width(lineNumWidth).TextAlign(ui.End).Shrink(0)
		ui.RichText(c, matchSpans(t, m, replacement, replacing, sel)...).FontSize(smallFont).SingleLine().Grow(1).MinWidth(0)
		if r.detail != "" {
			mutedText(c, r.detail, sel).SingleLine().TruncateStart().Shrink(1).MinWidth(0)
		}
		if lr.Hovered() || sel {
			v.rowActions(c, r, sel)
		}
	})
	el.ContextMenu(func(m *ui.Menu) { v.rowMenu(c, m, r) })
	if el.RowDoubleClicked() {
		v.activateRow(lr.Index, OpenOptions{Focus: true})
	}
}

func (v *View) rowActions(c *ui.Context, r row, sel bool) {
	if v.scope != ScopeText {
		return
	}
	key := r.key
	if v.st.ShowReplace && (r.kind == rowMatch || r.kind == rowFile) {
		if iconButtonOn(c, "replace-"+key, "Replace", icons.Replace, !v.canReplace(), sel).Clicked() {
			if r.kind == rowMatch {
				v.replaceMatch(r.file, r.match)
			} else {
				v.replaceFile(r.file)
			}
		}
	}
	if r.canDismiss() {
		if iconButtonOn(c, "dismiss-"+key, "Dismiss", icons.Close, false, sel).Clicked() {
			v.dismiss(r)
		}
	}
}

func (v *View) resultView(lr ui.ListRow, r row) {
	c := lr.Context
	t := c.Theme()
	sel := lr.Selected()
	res := r.res
	el := v.box(lr, r)
	el.Children(func() {
		switch res.Kind {
		case everywhere.KindFile:
			fileIcon(c, res.Label)
		case everywhere.KindSymbol:
			kindBadge(c, res, sel)
		case everywhere.KindGitRef:
			ui.Icon(c, icons.GitBranch).Size(iconSize, iconSize).TextColor(mutedColor(t, sel)).Shrink(0)
		case everywhere.KindGitCommit:
			ui.Icon(c, icons.Commit).Size(iconSize, iconSize).TextColor(mutedColor(t, sel)).Shrink(0)
		}
		ui.RichText(c, highlightSpans(t, res.Label, res.Positions, sel)...).FontSize(bodyFont).SingleLine().Shrink(0)
		v.resultDetail(c, res, sel)
		ui.Box(c).Grow(1)
		if res.Kind == everywhere.KindAction && res.Keys != "" {
			mutedText(c, res.Keys, sel).SingleLine().Shrink(0)
		}
		if res.Open {
			ui.Icon(c, icons.Dot).Size(8, 8).TextColor(mutedColor(t, sel)).Tooltip("Open in an editor").Shrink(0)
		}
	})
	el.ContextMenu(func(m *ui.Menu) { v.rowMenu(c, m, r) })
	if el.RowDoubleClicked() {
		v.activateRow(lr.Index, OpenOptions{Focus: true})
	}
}
