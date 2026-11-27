package finder

import (
	"errors"
	"slices"
	"strconv"
	"strings"

	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/picker"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/findwidget"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

const (
	dialogID         = "search"
	scopeLabel       = "Search scope"
	resultsLabel     = "Results"
	previewLabel     = "Preview"
	filterLabel      = "Filter"
	findWindowLabel  = "Open in Find Window"
	allTypesLabel    = "All File Types"
	nonProjectLabel  = "Include Non-Project Items"
	searchingText    = "Searching..."
	pageRows         = token.SearchPopupRows - 1
	labelEverywhere  = "Search Everywhere"
	labelActions     = "Command Palette"
	labelFiles       = "Go to File"
	labelText        = "Find in Files"
	labelSymbols     = "Go to Symbol in Workspace"
	labelEditorSyms  = "Go to Symbol in Editor"
	labelLine        = "Go to Line/Column"
	labelHelp        = "Help"
	placeholderAll   = "Search files, actions and text"
	placeholderActs  = "Type the name of a command to run"
	placeholderFiles = "Search files by name (type ? for help)"
	placeholderText  = "Type the text to search for"
)

func (d *Dialog) label() string {
	switch d.tab {
	case TabActions:
		return labelActions
	case TabText:
		if d.st.ShowReplace {
			return titleReplace
		}
		return labelText
	case TabFiles:
		q := strings.TrimLeft(d.st.Query, " ")
		switch {
		case strings.HasPrefix(q, prefixSymbols):
			return labelSymbols
		case strings.HasPrefix(q, prefixEditorSymbols):
			return labelEditorSyms
		case strings.HasPrefix(q, prefixLine):
			return labelLine
		case strings.HasPrefix(q, prefixHelp):
			return labelHelp
		}
		return labelFiles
	}
	return labelEverywhere
}

func (d *Dialog) placeholder() string {
	return [tabCount]string{placeholderAll, placeholderActs, placeholderFiles, placeholderText}[d.tab]
}

func (d *Dialog) queryError() string {
	if d.wantsText() {
		return d.text.err
	}
	return ""
}

func (d *Dialog) busy() bool {
	return d.loading || d.text.searching || d.replacing
}

func (d *Dialog) View(c *ui.Context) {
	if !d.open {
		return
	}
	d.pal = token.Of(c)
	if d.searchKey() != d.searched {
		d.refresh()
	}
	if d.text.version != d.shown {
		d.compose(d.text.takeRenewed())
	}
	d.showBusy = d.progress.visible(c, d.busy())
	label := d.label()
	opts := picker.DialogOptions{Label: label, ID: dialogID, Layout: &d.st.Layout, WindowKeys: d.env.WindowKeys,
		Shortcuts: append([]picker.Shortcut{{Mods: ui.Cmd | ui.Shift, Key: ui.Key1, Run: d.replaceSelected}}, d.toggleShortcuts()...)}
	if picker.Dialog(c, opts, func() { d.body(c, label) }) && !d.confirm {
		d.Close()
		return
	}
	d.confirmView(c)
}

func (d *Dialog) previewShown() bool {
	return d.st.Preview && d.hasPreview()
}

func (d *Dialog) hasPreview() bool {
	return d.tab == TabFiles || d.tab == TabText
}

func (d *Dialog) previewable(row int) bool {
	if row < 0 || row >= len(d.rowEntry) || d.rowEntry[row] == noEntry {
		return false
	}
	kind := d.entries[d.rowEntry[row]].kind
	return kind != kindMore && kind != kindHint
}

func (d *Dialog) body(c *ui.Context, label string) {
	tab := int(d.tab)
	picker.Header(c, func() { picker.Tabs(c, scopeLabel, &tab, tabTitles[:]) }, func() { d.headerActions(c) })
	if tab != int(d.tab) {
		d.setTab(Tab(tab))
	}
	var toggles = findwidget.Toggles(&d.st.MatchCase, &d.st.WholeWord, &d.st.Regex)
	if d.tab != TabText {
		toggles = nil
	}
	keys := d.queryRow(c, label, toggles)
	if d.selectAll {
		d.field.SelectAll()
		d.selectAll = false
	}
	d.handleKeys(c, keys)
	if !d.open {
		return
	}
	if d.tab == TabText {
		d.optionsRow(c)
	}
	results := picker.ResultsOptions{Label: resultsLabel, Fill: true, Empty: d.emptyState}
	clicked := noEntry
	if d.previewShown() {
		results.OnSelect = func(row int) {
			d.moved = true
			if !d.previewable(row) {
				clicked = row
			}
		}
	}
	i := picker.Results(c, &d.results, d.rows, results)
	if i < 0 {
		i = clicked
	}
	if i >= 0 {
		d.moved = true
		d.results.Selected = i
		d.accept(i, false)
	}
	if d.previewShown() && d.open {
		d.syncPreview()
		d.preview.pane(c, &d.st.Layout, &d.results)
	}
}

func (d *Dialog) handleKeys(c *ui.Context, k picker.Keys) {
	switch {
	case k.Next:
		d.cycleTab(1)
		c.Invalidate()
	case k.Previous:
		d.cycleTab(-1)
		c.Invalidate()
	case k.Down:
		d.move(1)
	case k.Up:
		d.move(-1)
	case k.PageDown:
		d.move(pageRows)
	case k.PageUp:
		d.move(-pageRows)
	case k.Side:
		d.accept(d.results.Selected, true)
	case k.Submitted:
		d.accept(d.results.Selected, false)
	}
	if k.Changed {
		d.queryChanged()
		c.Invalidate()
	}
}

func (d *Dialog) move(delta int) {
	d.moved = true
	d.results.Move(delta)
}

func (d *Dialog) syncPreview() {
	e, ok := d.current()
	if !ok && d.busy() {
		return
	}
	if !ok {
		d.preview.show(Location{})
		return
	}
	loc, ok := e.location()
	if !ok {
		d.preview.show(Location{})
		return
	}
	d.preview.show(loc)
}

func (d *Dialog) headerActions(c *ui.Context) {
	if d.tab != TabActions {
		action.MenuIconButton(c, icons.Filter, filterLabel, action.IconOptions{Active: len(d.st.FileTypes) > 0 || d.st.NonProject}, d.filterMenu)
	}
	if d.hasPreview() {
		action.ToggleIconButton(c, icons.Preview, previewLabel, &d.st.Preview, action.IconOptions{})
	}
	if action.IconButton(c, icons.TreeView, findWindowLabel, action.IconOptions{Disabled: !d.canOpenInFind()}).Clicked() {
		d.openInFindWindow()
	}
}

func (d *Dialog) filterMenu(m *ui.Menu) {
	if action.Item(m, allTypesLabel, action.ItemOptions{Checked: len(d.st.FileTypes) == 0}).Chosen() {
		d.st.FileTypes = nil
	}
	for _, t := range d.fileTypes() {
		if action.Item(m, t.name, action.ItemOptions{Checked: slices.Contains(d.st.FileTypes, t.name), Detail: strconv.Itoa(t.count)}).Chosen() {
			d.toggleFileType(t.name)
		}
	}
	m.Separator()
	if action.Item(m, nonProjectLabel, action.ItemOptions{Checked: d.st.NonProject}).Chosen() {
		d.st.NonProject = !d.st.NonProject
	}
}

func (d *Dialog) emptyState(c *ui.Context) {
	q := strings.TrimSpace(d.st.Query)
	switch {
	case d.busy() && d.showBusy:
		feedback.Loading(c, searchingText)
	case d.busy():
		if d.settled != "" {
			feedback.Empty(c, feedback.EmptyOptions{Icon: icons.Search, Title: d.settled})
		}
	case d.queryError() != "":
		d.settled = ""
		feedback.Error(c, "The pattern is not valid", errors.New(d.queryError()), nil)
	case q == "":
		d.settled = d.hint()
		feedback.Empty(c, feedback.EmptyOptions{Icon: icons.Search, Title: d.settled})
	default:
		d.settled = "No results for " + quote(firstLine(q))
		feedback.Empty(c, feedback.EmptyOptions{Icon: icons.Search, Title: d.settled})
	}
}

func (d *Dialog) hint() string {
	return [tabCount]string{"Type to search files, actions and text", "Type the name of a command", "Type a file name. Add :line to jump to a line", "Type to search the text of your files"}[d.tab]
}

func quote(s string) string { return strconv.Quote(s) }

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}

type FindResults struct {
	Title string
	Files []*FileNode
	Total int
}
