package finder

import (
	"errors"
	"strconv"
	"strings"

	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/dialog"
	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/field"
	"github.com/ZerGo0/editor/internal/design/list"
	"github.com/ZerGo0/editor/internal/design/picker"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/findwidget"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/ZerGo0/editor/internal/search/textsearch"
	"github.com/egoist/mygo/ui"
)

const (
	replaceID          = "replace-in-files"
	titleFind          = "Find in Files"
	titleReplace       = "Replace in Files"
	searchLabel        = "Search"
	replaceLabel       = "Replace"
	replaceAllLabel    = "Replace All"
	toggleReplaceLabel = "Toggle Replace"
	maskLabel          = "File mask"
	maskPlaceholder    = "File mask, for example *.go, *.md"
	scopeSelectLabel   = "Scope"
	directoryLabel     = "Directory"
	directoryHint      = "Directory path"
	matchesLabel       = "Matches"
	confirmHint        = "Open files change in their editors and stay unsaved. Other files are written to disk."
	cancelLabel        = "Cancel"
)

type ReplacePopup struct {
	env       Env
	st        ReplaceState
	open      bool
	text      textRun
	rows      []findRow
	collapsed map[string]bool
	dirty     bool
	list      ui.ListState
	selected  int
	preview   preview
	searched  string
	shown     int
	replacing bool
	confirm   bool
	focus     bool
}

func NewReplacePopup(env Env) *ReplacePopup {
	r := &ReplacePopup{env: env, text: textRun{env: env}, preview: preview{env: env}, collapsed: map[string]bool{}, selected: -1, dirty: true}
	r.list.Selected = &r.selected
	r.preview.clear()
	return r
}

func (r *ReplacePopup) IsOpen() bool { return r.open }

func (r *ReplacePopup) Snapshot() ReplaceState { return r.st }

func (r *ReplacePopup) Restore(st ReplaceState) { r.st = st }

func (r *ReplacePopup) Open(seed string) {
	r.open, r.focus, r.confirm = true, true, false
	r.st.ShowReplace = true
	if seed != "" {
		r.st.Query = seed
	}
	r.refresh()
}

func (r *ReplacePopup) Close() {
	r.open, r.confirm = false, false
	r.text.reset()
	r.preview.clear()
}

func (r *ReplacePopup) key() string {
	parts := []string{r.st.Query, strconv.FormatBool(r.st.MatchCase), strconv.FormatBool(r.st.WholeWord), strconv.FormatBool(r.st.Regex),
		r.st.Mask, r.st.Scope, r.st.Directory}
	return strings.Join(parts, "\x00")
}

func (r *ReplacePopup) directory() string {
	if r.st.Scope == ScopeDirectory {
		return strings.TrimSpace(r.st.Directory)
	}
	return ""
}

func (r *ReplacePopup) request() textRequest {
	return textRequest{Pattern: r.st.Query, MatchCase: r.st.MatchCase, WholeWord: r.st.WholeWord, Regex: r.st.Regex,
		Include: textsearch.ParseGlobs(r.st.Mask), Directory: r.directory()}
}

func (r *ReplacePopup) refresh() {
	r.searched = r.key()
	r.selected, r.dirty = -1, true
	if r.st.Scope == ScopeDirectory && r.directory() == "" {
		r.text.reset()
		return
	}
	r.text.start(r.request())
}

func (r *ReplacePopup) build() {
	if !r.dirty && r.shown == r.text.version {
		return
	}
	r.dirty, r.shown = false, r.text.version
	r.rows = r.rows[:0]
	for _, file := range r.text.files {
		r.rows = append(r.rows, findRow{file: file, match: -1})
		if r.collapsed[file.Path] {
			continue
		}
		for i := range file.Matches {
			r.rows = append(r.rows, findRow{file: file, match: i})
		}
	}
	if r.selected < 0 || r.selected >= len(r.rows) {
		r.selected = firstMatchRow(r.rows)
	}
}

func firstMatchRow(rows []findRow) int {
	for i, row := range rows {
		if row.match >= 0 {
			return i
		}
	}
	if len(rows) > 0 {
		return 0
	}
	return -1
}

func (r *ReplacePopup) current() (findRow, bool) {
	if r.selected < 0 || r.selected >= len(r.rows) {
		return findRow{}, false
	}
	return r.rows[r.selected], true
}

func (r *ReplacePopup) title() string {
	if r.st.ShowReplace {
		return titleReplace
	}
	return titleFind
}

func (r *ReplacePopup) View(c *ui.Context) {
	if !r.open {
		return
	}
	if r.searched != r.key() {
		r.refresh()
	}
	r.build()
	if picker.Dialog(c, picker.DialogOptions{Label: r.title(), ID: replaceID}, func() { r.body(c) }) && !r.confirm {
		r.Close()
		return
	}
	r.confirmView(c)
}

func (r *ReplacePopup) body(c *ui.Context) {
	p := token.Of(c)
	picker.Header(c, func() {
		token.TextStrong.Apply(ui.Text(c, r.title())).TextColor(p.Text.Primary).SingleLine().Role(ui.RoleHeading).Level(1)
	}, nil)
	r.fields(c)
	r.options(c)
	r.resultsView(c, p)
	if r.open {
		r.syncPreview()
		picker.Preview(c, r.preview.st, picker.PreviewOptions{Label: previewLabel, Message: r.preview.message, Busy: r.preview.loading})
		r.footer(c, p)
	}
}

func (r *ReplacePopup) fields(c *ui.Context) {
	ui.Row(c).FillWidth().AlignItems(ui.Start).Gap(token.GapInline.Px()).Children(func() {
		if action.ExpandButton(c, toggleReplaceLabel, r.st.ShowReplace, action.IconOptions{}).Clicked() {
			r.st.ShowReplace = !r.st.ShowReplace
		}
		ui.Column(c).Grow(1).MinWidth(0).Gap(token.GapInline.Px()).Children(func() {
			toggles := findwidget.Toggles(&r.st.MatchCase, &r.st.WholeWord, &r.st.Regex)
			_, submit := findwidget.Field(c, searchLabel, &r.st.Query, findwidget.FieldOptions{Error: r.text.err, Toggles: toggles, AutoFocus: r.focus})
			r.focus = false
			if submit == findwidget.Enter {
				r.openSelected()
			}
			if !r.st.ShowReplace {
				return
			}
			_, submit = findwidget.Field(c, replaceLabel, &r.st.With, findwidget.FieldOptions{Toggles: []field.Toggle{findwidget.PreserveCase(&r.st.PreserveCase)}})
			switch submit {
			case findwidget.CmdEnter:
				r.askReplaceAll()
			case findwidget.Enter:
				r.replaceSelected()
			}
		})
	})
}

func (r *ReplacePopup) options(c *ui.Context) {
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(token.GapControl.Px()).Children(func() {
		mask := field.TextInput(c, maskLabel, &r.st.Mask, field.InputOptions{Placeholder: maskPlaceholder})
		mask.Grow(1).MinWidth(0)
		scopes := []action.Option[string]{{Value: ScopeProject, Label: ScopeProject}, {Value: ScopeDirectory, Label: ScopeDirectory}}
		if r.st.Scope == "" {
			r.st.Scope = ScopeProject
		}
		action.Select(c, scopeSelectLabel, &r.st.Scope, scopes, action.SelectOptions{})
		if r.st.Scope == ScopeDirectory {
			field.TextInput(c, directoryLabel, &r.st.Directory, field.InputOptions{Placeholder: directoryHint}).Grow(1).MinWidth(0)
		}
	})
}

func (r *ReplacePopup) resultsView(c *ui.Context, p *token.Palette) {
	height := token.RowHeight.Px() * float32(token.SearchPopupRowsWithPreview)
	el := list.View(c, &r.list, len(r.rows), list.ViewOptions{Label: matchesLabel, Tree: true, Height: height, Empty: r.emptyState},
		func(i int) list.Item { return r.item(p, i) })
	if el.Submitted() {
		r.openSelected()
	}
}

func (r *ReplacePopup) emptyState(c *ui.Context) {
	q := strings.TrimSpace(r.st.Query)
	switch {
	case r.text.searching:
		feedback.Loading(c, searchingText)
	case r.text.err != "":
		feedback.Error(c, "The pattern is not valid", errors.New(r.text.err), nil)
	case r.st.Scope == ScopeDirectory && r.directory() == "":
		feedback.Empty(c, feedback.EmptyOptions{Icon: icons.Folder, Title: "Type a directory to search in"})
	case q == "":
		feedback.Empty(c, feedback.EmptyOptions{Icon: icons.Search, Title: "Type the text to search for"})
	default:
		feedback.Empty(c, feedback.EmptyOptions{Icon: icons.Search, Title: "No results for " + quote(firstLine(q))})
	}
}

func (r *ReplacePopup) item(p *token.Palette, i int) list.Item {
	row := r.rows[i]
	file := row.file
	if row.match < 0 {
		return list.Item{Label: file.Name, Description: file.Dir, Leading: fileIcon(file.Name), Count: len(file.Matches), Strong: true, Branch: true,
			Open: !r.collapsed[file.Path], OnClick: func() { r.toggle(file) }}
	}
	m := file.Matches[row.match]
	replacing, replacement := r.st.ShowReplace && r.text.matcher != nil, ""
	if replacing {
		replacement = r.replacement().Expand(m.Matched, m.Groups)
	}
	return list.Item{Label: lineLabel(m), Depth: 1, Description: "line " + strconv.Itoa(m.Start.Line+1),
		Rich: func(s list.Style) []ui.Span { return matchSpans(p, s, m, replacement, replacing) }, OnDoubleClick: r.openSelected}
}

func (r *ReplacePopup) toggle(file *FileNode) {
	r.collapsed[file.Path] = !r.collapsed[file.Path]
	r.dirty = true
}

func (r *ReplacePopup) syncPreview() {
	row, ok := r.current()
	if !ok {
		r.preview.show(Location{})
		return
	}
	match := row.match
	if match < 0 && len(row.file.Matches) > 0 {
		match = 0
	}
	r.preview.show(row.file.location(match))
}

func (r *ReplacePopup) openSelected() {
	row, ok := r.current()
	if !ok {
		return
	}
	match := row.match
	if match < 0 && len(row.file.Matches) > 0 {
		match = 0
	}
	loc := row.file.location(match)
	r.Close()
	r.env.open(loc, OpenOptions{Focus: true})
}

func (r *ReplacePopup) openInFindWindow() {
	res := FindResults{Title: "Text matches for " + quote(r.st.Query), Files: r.text.files, Total: r.text.total}
	r.Close()
	if r.env.ShowFindResults != nil {
		r.env.ShowFindResults(res)
	}
}

func (r *ReplacePopup) status() string {
	switch {
	case r.replacing:
		return "Replacing..."
	case r.text.searching:
		return searchingText
	case r.text.total == 0:
		return ""
	}
	status := plural(r.text.total, "match") + " in " + plural(len(r.text.files), "file")
	if r.text.truncated {
		status += " (first results only)"
	}
	return status
}

func (r *ReplacePopup) footer(c *ui.Context, p *token.Palette) {
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(token.GapControl.Px()).Children(func() {
		token.TextLabel.Apply(ui.Text(c, r.status())).TextColor(p.Text.Secondary).SingleLine().Grow(1).MinWidth(0).Role(ui.RoleStatus)
		if action.Button(c, findWindowLabel, action.ButtonOptions{Variant: action.Ghost, Disabled: r.text.total == 0 || r.text.searching}).Clicked() {
			r.openInFindWindow()
		}
		if !r.st.ShowReplace {
			return
		}
		_, selected := r.current()
		if action.Button(c, replaceLabel, action.ButtonOptions{Disabled: !r.canReplace() || !selected}).Clicked() {
			r.replaceSelected()
		}
		if action.Button(c, replaceAllLabel, action.ButtonOptions{Variant: action.Primary, Disabled: !r.canReplace()}).Clicked() {
			r.askReplaceAll()
		}
	})
}

func (r *ReplacePopup) askReplaceAll() {
	r.confirm = r.canReplace()
}

func (r *ReplacePopup) confirmView(c *ui.Context) {
	if !r.confirm {
		return
	}
	title := replaceAllLabel
	hint := "Replace " + plural(r.text.total, "occurrence") + " across " + plural(len(r.text.files), "file") + "? " + confirmHint
	dialog.Modal(c, &r.confirm, title, dialog.Options{Hint: hint}, func() {
		var cancel, ok ui.Element
		dialog.Actions(c, func() {
			cancel = action.Button(c, cancelLabel, action.ButtonOptions{ID: "replace-cancel"})
			ok = action.Button(c, replaceAllLabel, action.ButtonOptions{Variant: action.Primary, ID: "replace-confirm"})
		})
		switch {
		case cancel.Clicked():
			r.confirm = false
		case ok.Clicked():
			r.confirm = false
			r.replaceAll()
		}
	})
}
