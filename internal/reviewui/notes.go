package reviewui

import (
	"fmt"
	"strings"

	"github.com/ZerGo0/editor/internal/design"
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/field"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/git/diffmode"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/ZerGo0/editor/internal/multidiff"
	"github.com/ZerGo0/editor/internal/review"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	commentLabel       = "Comment"
	suggestionLabel    = "Suggested Code"
	addSuggestionLabel = "Add Suggested Code"
	commentPlaceholder = "Leave feedback..."
	addCommentLabel    = "Add Comment"
	saveCommentLabel   = "Save Comment"
	cancelLabel        = "Cancel"
	editLabel          = "Edit Annotation"
	deleteLabel        = "Delete Annotation"
	fileCommentLabel   = "File Comment"
	revealTooltip      = "Show in Diff"
	submitKeys         = "\u2318\u21a9"
	cancelKeys         = "Esc"
	composerMinLines   = 2
	composerMaxLines   = 10
	estimatedRows      = 4
)

type note struct {
	id int
	review.Annotation
}

type noteKey struct{ id int }

type draftKey struct{}

type draft struct {
	open       bool
	editing    int
	ann        review.Annotation
	text       string
	suggestion string
	suggest    bool
}

func (d *draft) empty() bool {
	return strings.TrimSpace(d.text) == "" && (!d.suggest || strings.TrimSpace(d.suggestion) == "")
}

func (t *Tab) annotations() []review.Annotation {
	out := make([]review.Annotation, len(t.notes))
	for i, n := range t.notes {
		out[i] = n.Annotation
	}
	return out
}

func (t *Tab) save(d *draft) {
	if d.empty() {
		return
	}
	a := d.ann
	a.Text, a.Suggestion = strings.TrimSpace(d.text), ""
	if d.suggest && strings.TrimSpace(d.suggestion) != "" {
		a.Suggestion = strings.TrimRight(d.suggestion, "\n")
	}
	if d.editing != 0 {
		for i := range t.notes {
			if t.notes[i].id == d.editing {
				t.notes[i].Annotation = a
			}
		}
	} else {
		t.nextID++
		t.notes = append(t.notes, note{id: t.nextID, Annotation: a})
	}
	*d = draft{}
	t.clearSelection(a.Path)
}

func (t *Tab) remove(id int) {
	for i, n := range t.notes {
		if n.id == id {
			t.notes = append(t.notes[:i:i], t.notes[i+1:]...)
			delete(t.heights, noteKey{id})
			return
		}
	}
}

func (t *Tab) edit(n note) {
	d := draft{open: true, editing: n.id, ann: n.Annotation, text: n.Text, suggestion: n.Suggestion, suggest: n.Suggestion != ""}
	if n.Scope == review.ScopeLine {
		t.inline = d
		t.reveal(n.Annotation)
		return
	}
	t.panel = d
}

func (t *Tab) compose(path string, d *editor.DiffState, r editor.DiffLineRange) {
	if t.done {
		return
	}
	buf, side := d.New, review.SideNew
	if r.Side == editor.DiffOld {
		buf, side = d.Old, review.SideOld
	}
	if buf == nil {
		return
	}
	a := review.Annotation{Scope: review.ScopeLine, Path: path, Side: side, Start: r.Start + 1, End: r.End + 1, Quote: quote(buf, r.Start, r.End)}
	if t.inline.open && t.inline.editing == 0 {
		t.inline.ann = a
		return
	}
	t.inline = draft{open: true, ann: a}
}

func quote(buf *editor.Buffer, start, end int) string {
	lines := make([]string, 0, end-start+1)
	for i := start; i <= end && i < buf.LineCount(); i++ {
		lines = append(lines, strings.TrimRight(buf.Line(i), "\r\n"))
	}
	return strings.Join(lines, "\n")
}

func (t *Tab) decorate(entries []multidiff.Entry) []multidiff.Entry {
	for i := range entries {
		e := &entries[i]
		if e.Section != nil || e.Path == "" {
			continue
		}
		path, setup, rows, actions := e.Path, e.Setup, e.Annotations, e.Actions
		e.Setup = func(d *editor.DiffState, eng *diffmode.Engine) {
			if setup != nil {
				setup(d, eng)
			}
			t.states[path] = d
			d.LineSelection = true
			d.OnLineSelection = func(r editor.DiffLineRange) { t.compose(path, d, r) }
			d.OnGutterButton = func(r editor.DiffLineRange) { t.compose(path, d, r) }
			eng.Install(func(c *ui.Context, a editor.DiffAnnotation) { t.row(c, a) })
		}
		e.Annotations = func(d *editor.DiffState) []editor.DiffAnnotation {
			var base []editor.DiffAnnotation
			if rows != nil {
				base = rows(d)
			}
			return append(base, t.rows(path, d)...)
		}
		e.Actions = func(c *ui.Context) {
			if action.IconButton(c, icons.Comment, fileCommentLabel, action.IconOptions{Disabled: t.done, ID: "file-comment"}).Clicked() {
				t.panel = draft{open: true, ann: review.Annotation{Scope: review.ScopeFile, Path: path}}
			}
			if actions != nil {
				actions(c)
			}
		}
	}
	return entries
}

func (t *Tab) rows(path string, d *editor.DiffState) []editor.DiffAnnotation {
	var out []editor.DiffAnnotation
	add := func(a review.Annotation, key any) {
		side, buf := editor.DiffNew, d.New
		if a.Side == review.SideOld {
			side, buf = editor.DiffOld, d.Old
		}
		line := a.End - 1
		if buf != nil {
			line = min(line, buf.LineCount()-1)
		}
		out = append(out, editor.DiffAnnotation{Side: side, Line: max(line, 0), Height: t.height(key), Key: key})
	}
	for _, n := range t.notes {
		if n.Scope == review.ScopeLine && n.Path == path && !(t.inline.open && t.inline.editing == n.id) {
			add(n.Annotation, noteKey{n.id})
		}
	}
	if t.inline.open && t.inline.ann.Path == path {
		add(t.inline.ann, draftKey{})
	}
	return out
}

func (t *Tab) height(key any) float32 {
	if h, ok := t.heights[key]; ok {
		return h
	}
	return token.ControlHeight.Px() * estimatedRows
}

func (t *Tab) row(c *ui.Context, a editor.DiffAnnotation) {
	ui.Column(c).Fill().ClipY().Children(func() {
		body := ui.Column(c.Key(fmt.Sprint(a.Key))).FillWidth().Padding(token.GapTight.Px(), token.PadPanel.Px())
		body.Children(func() {
			switch k := a.Key.(type) {
			case draftKey:
				t.composer(c, &t.inline, review.LineLabel(t.inline.ann.Start, t.inline.ann.End))
			case noteKey:
				for _, n := range t.notes {
					if n.id == k.id {
						t.card(c, n, false)
					}
				}
			}
		})
		if h := body.LayoutBounds().H; h > 0 && h != t.heights[a.Key] {
			t.heights[a.Key] = h
			c.Invalidate()
		}
	})
}

func (t *Tab) composer(c *ui.Context, d *draft, title string) {
	p := token.Of(c)
	submit, cancel := func() { t.save(d) }, func() { *d = draft{} }
	ui.Column(c.Key("composer")).FillWidth().Gap(token.GapInline.Px()).Padding(token.GapControl.Px(), token.PadPanel.Px()).
		Background(p.Layer.Widget).Border(token.BorderWidth.Px(), p.Accent.Focus).Radius(token.RadiusCard.Px()).
		Role(ui.RoleGroup).Label(title).OnShortcut(ui.Cmd, ui.KeyEnter, submit).OnShortcut(0, ui.KeyEscape, cancel).Children(func() {
		design.Text(c, title, token.TextStrong)
		field.TextArea(c, commentLabel, &d.text, field.AreaOptions{Placeholder: commentPlaceholder, MinLines: composerMinLines, MaxLines: composerMaxLines, AutoFocus: true, ID: "comment"})
		if d.suggest {
			field.TextArea(c, suggestionLabel, &d.suggestion, field.AreaOptions{MinLines: composerMinLines, MaxLines: composerMaxLines, Mono: true, ID: "suggestion"})
		} else if d.ann.Scope == review.ScopeLine {
			if action.Button(c, addSuggestionLabel, action.ButtonOptions{Variant: action.Ghost, Small: true, Icon: icons.Plus, ID: "add-suggestion"}).Clicked() {
				d.suggest, d.suggestion = true, d.ann.Quote
			}
		}
		ui.Row(c).FillWidth().Justify(ui.End).Gap(token.GapControl.Px()).Children(func() {
			if action.Button(c, cancelLabel, action.ButtonOptions{Small: true, Keys: cancelKeys, ID: "cancel"}).Clicked() {
				cancel()
			}
			label := addCommentLabel
			if d.editing != 0 {
				label = saveCommentLabel
			}
			if action.Button(c, label, action.ButtonOptions{Variant: action.Primary, Small: true, Keys: submitKeys, Disabled: d.empty(), ID: "submit"}).Clicked() {
				submit()
			}
		})
	})
}

func (t *Tab) card(c *ui.Context, n note, located bool) {
	p := token.Of(c)
	ui.Column(c.Key(n.id)).FillWidth().Gap(token.GapInline.Px()).Padding(token.GapControl.Px(), token.PadPanel.Px()).
		Background(p.Layer.Panel).Border(token.BorderWidth.Px(), p.Border.Default).Radius(token.RadiusCard.Px()).
		Role(ui.RoleGroup).Label(n.Short() + " " + n.Text).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(token.GapTight.Px()).Children(func() {
			if located && n.Scope == review.ScopeLine {
				if action.Button(c, n.Short(), action.ButtonOptions{Variant: action.Ghost, Small: true, Tooltip: revealTooltip, ID: "reveal"}).Clicked() {
					t.reveal(n.Annotation)
				}
			} else {
				design.Text(c, n.Short(), token.TextStrong)
			}
			design.Spacer(c)
			if action.IconButton(c, icons.Edit, editLabel, action.IconOptions{Disabled: t.done, ID: "edit"}).Clicked() {
				t.edit(n)
			}
			if action.IconButton(c, icons.Close, deleteLabel, action.IconOptions{Disabled: t.done, ID: "delete"}).Clicked() {
				t.remove(n.id)
			}
		})
		if n.Text != "" {
			design.Text(c, n.Text, token.TextBody).Wrap().Selectable()
		}
		if n.Suggestion != "" {
			design.Text(c, n.Suggestion, token.TextMono).Wrap().Selectable().FillWidth().Background(p.Layer.Input).
				Radius(token.RadiusControl.Px()).Padding(token.PadInputY.Px(), token.PadInputX.Px())
		}
	})
}

func (t *Tab) reveal(a review.Annotation) {
	if t.src == nil {
		return
	}
	t.src.View().RevealPath(a.Path)
	if d := t.states[a.Path]; d != nil && a.Scope == review.ScopeLine {
		side := editor.DiffNew
		if a.Side == review.SideOld {
			side = editor.DiffOld
		}
		d.SetSelectedLines(editor.DiffLineRange{Side: side, Start: a.Start - 1, End: a.End - 1}, true)
	}
}

func (t *Tab) clearSelection(path string) {
	if d := t.states[path]; d != nil {
		d.SetSelectedLines(editor.DiffLineRange{}, false)
	}
}
