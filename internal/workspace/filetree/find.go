package filetree

import (
	"strings"

	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/field"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/fuzzy"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

const (
	findLabel       = "Filter Files"
	findPlaceholder = "Filter"
	findCloseLabel  = "Close Filter"
	findCloseKeys   = "Esc"
	findID          = "tree-find"
	noMatchesTitle  = "No matching files"
)

type finder struct {
	query   string
	applied string
	focus   bool
}

func (t *Tree) StartFind() {
	if t.find == nil {
		t.find = &finder{}
	}
	t.find.focus = true
}

func (t *Tree) Finding() bool {
	return t.find != nil
}

func (t *Tree) closeFind() {
	t.find = nil
	t.state.Generation++
	t.focus = true
}

func (t *Tree) filter() string {
	if t.find == nil {
		return ""
	}
	return strings.TrimSpace(t.find.query)
}

func (t *Tree) findBar(c *ui.Context) {
	f := t.find
	closing := false
	ui.Row(c).FillWidth().Padding(token.GapInline.Px(), token.PadRow.Px()).Shrink(0).Children(func() {
		in := field.TextInput(c, findLabel, &f.query, field.InputOptions{Placeholder: findPlaceholder, Icon: icons.Search, ID: findID,
			Actions: []field.Action{{Icon: icons.Close, Label: findCloseLabel, Keys: findCloseKeys, Run: func() { closing = true }}}}).Grow(1).MinWidth(0)
		if f.focus {
			f.focus = false
			in.Focus()
		}
		switch {
		case in.Shortcut(0, ui.KeyEscape):
			closing = true
		case in.Shortcut(0, ui.KeyDown) || in.Submitted():
			t.focusFirstMatch()
		}
	})
	if closing {
		t.closeFind()
		return
	}
	if q := t.filter(); q != f.applied {
		f.applied = q
		t.state.Generation++
	}
}

func (t *Tree) focusFirstMatch() {
	t.focus = true
	q := t.filter()
	for i := range t.state.Rows() {
		if _, ok := fuzzy.Find(q, t.name(t.state.Item(i))); ok || q == "" {
			t.choose(i)
			t.state.List.ScrollIntoView(i)
			return
		}
	}
}

func (t *Tree) hidden(p string) bool {
	q := t.filter()
	if q == "" || t.isPlaceholder(p) {
		return false
	}
	if _, ok := fuzzy.Find(q, t.name(p)); ok {
		return false
	}
	return !t.isDir[p] || !t.state.Open.Has(p) || len(t.visibleRows(p)) == 0
}

func (t *Tree) matches(p string) []int {
	q := t.filter()
	if q == "" {
		return nil
	}
	m, _ := fuzzy.Find(q, t.name(p))
	return m.Positions
}

func (t *Tree) noMatches(c *ui.Context) {
	feedback.Empty(c, feedback.EmptyOptions{Icon: icons.Filter, Title: noMatchesTitle})
}
