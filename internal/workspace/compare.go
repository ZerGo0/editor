package workspace

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/design"
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/pane"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/document"
	"github.com/ZerGo0/editor/internal/git/diffmode"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/ZerGo0/editor/internal/multidiff"
	"github.com/ZerGo0/editor/internal/plural"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	compareSavedPrefix     = "compare-saved:"
	compareClipboardPrefix = "compare-clipboard:"
	compareToolbarLabel    = "Compare Toolbar"
	compareLoadingLabel    = "Loading comparison"
	compareArrow           = " \u2194 "
	previousChangeLabel    = "Previous Change"
	nextChangeLabel        = "Next Change"
)

type compareTab struct {
	path     string
	saved    bool
	live     *editor.Buffer
	state    *editor.DiffState
	engine   *diffmode.Engine
	modified bool
}

func (w *Workspace) activeTextDoc() *document.Document {
	d := w.docOf(w.active.ActiveTab())
	if d == nil || !d.Editable() {
		return nil
	}
	return d
}

func (w *Workspace) CompareWithSaved() {
	if d := w.activeTextDoc(); d != nil {
		w.compareWithSaved(d)
	}
}

func (w *Workspace) compareWithSaved(d *document.Document) {
	if !onDisk(d.Path) {
		return
	}
	path, name := d.Path, filepath.Base(d.Path)
	applog.Go(func() {
		data, err := os.ReadFile(path)
		w.update(func() {
			if err != nil {
				w.showError("Cannot compare "+name, err)
				return
			}
			w.openCompare(compareSavedPrefix+path, name+" (in file)"+compareArrow+name, path, string(data), true)
		})
	})
}

func (w *Workspace) CompareWithClipboard() {
	d := w.activeTextDoc()
	if d == nil {
		return
	}
	w.openCompare(compareClipboardPrefix+d.Path, "Clipboard"+compareArrow+w.tabTitle(d.Path), d.Path, mygo.Clipboard.ReadText(), false)
}

func (w *Workspace) openCompare(id, title, path, old string, saved bool) {
	live := w.Buffer(path)
	if live == nil {
		return
	}
	ct := &compareTab{path: path, saved: saved, live: live, state: &editor.DiffState{}}
	ct.engine = diffmode.NewEngine(w.ctx, ct.state, w.update)
	ct.engine.Install(nil)
	ct.engine.SetSource(diffmode.Source{Old: old, New: live.String(), Live: live, Language: live.Language()})
	p := virtualPrefix + id
	if prev := w.virtual[p]; prev != nil && prev.close != nil {
		prev.close()
	}
	w.virtual[p] = &virtualTab{title: title, view: func(c *ui.Context) { w.compareView(c, ct) }, close: ct.engine.Close}
	w.OpenFile(p, false, true)
}

func (w *Workspace) compareView(c *ui.Context, ct *compareTab) {
	if w.Buffer(ct.path) != ct.live {
		placeholder(c, "This file is closed", "Open "+filepath.Base(ct.path)+" again to compare it.")
		return
	}
	w.syncSavedSide(ct)
	ct.engine.Refresh()
	ui.Column(c).Fill().Children(func() {
		w.compareToolbar(c, ct)
		if !ct.engine.Ready() {
			feedback.DiffSkeleton(c, compareLoadingLabel, multidiff.CountUnknown, multidiff.CountUnknown, func(d *editor.DiffState) { w.ConfigureDiff(d, ct.path) })
			return
		}
		w.ConfigureDiff(ct.state, ct.path)
		editor.DiffView(c.Key("compare"), ct.state).Grow(1).FillWidth().MinHeight(0)
	})
}

func (w *Workspace) syncSavedSide(ct *compareTab) {
	d := w.doc(ct.path)
	if !ct.saved || d == nil {
		return
	}
	modified := d.Modified()
	if ct.modified && !modified {
		ct.engine.SetSource(diffmode.Source{Old: ct.live.String(), New: ct.live.String(), Live: ct.live, Language: ct.live.Language()})
	}
	ct.modified = modified
}

func (w *Workspace) compareToolbar(c *ui.Context, ct *compareTab) {
	pane.Toolbar(c, compareToolbarLabel, func(c *ui.Context) {
		ui.Row(c).FillWidth().MinWidth(0).Padding(0, token.PadRow.Px()).Gap(token.GapControl.Px()).AlignItems(ui.Center).Children(func() {
			design.Text(c, w.rel(ct.path), token.TextLabel).SingleLine().Shrink(1).MinWidth(0).TruncateStart().Tooltip(ct.path)
			design.Spacer(c)
			d := ct.state
			hunks := d.Hunks()
			i, _ := d.CurrentHunk()
			label := plural.Count(len(hunks), "change", "changes")
			switch {
			case i >= 0 && len(hunks) > 0:
				label = fmt.Sprintf("%d of %d", i+1, len(hunks))
			case len(hunks) == 1:
				label = "1 change"
			}
			design.Text(c, label, token.TextLabel).SingleLine().Shrink(0)
			if action.IconButton(c, icons.ChevronUp, previousChangeLabel, action.IconOptions{Disabled: i <= 0}).Clicked() {
				d.GoToHunk(i - 1)
			}
			if action.IconButton(c, icons.ChevronDown, nextChangeLabel, action.IconOptions{Disabled: i >= len(hunks)-1}).Clicked() {
				d.GoToHunk(i + 1)
			}
		})
	})
}
