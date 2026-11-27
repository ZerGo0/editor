package multidiff

import (
	"errors"
	"strconv"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/filediff"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/git/diffmode"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	loadDiffLabel    = "Load Diff"
	largeNote        = "Large diffs are not rendered by default."
	binaryNote       = "Binary file not shown."
	emptyNote        = "This file has no content changes."
	whitespaceNote   = "Only whitespace changed."
	loadingLabel     = "Loading diff"
	diffLabelPrefix  = "Diff of "
	diffsLabelPrefix = "Diffs of "
)

func (v *View) Render(c *ui.Context, o Options) {
	v.frame++
	v.builds = 0
	v.opts = o
	if v.dirty {
		v.layoutRows()
	}
	v.updateTop()
	if v.revealPath != "" {
		if key, ok := v.keyOfPath(v.revealPath); ok {
			v.reveal, v.revealPath = key, ""
		}
	}
	if v.reveal != "" {
		if i, ok := v.headerRow[v.reveal]; ok {
			v.list.ScrollTo(i, ui.Start)
		}
		v.reveal = ""
	}
	root := ui.Column(c.Key("multidiff")).Fill()
	v.width = root.Bounds().W
	root.Children(func() {
		v.toolbar(c, o)
		if v.treeShown() {
			ui.Split(c, &v.tree.width, func() { v.sidebar(c) }, func() { v.stack(c, o) }).SplitReset(token.SidebarMinWidth.Px()).Grow(1).FillWidth().MinHeight(0)
		} else {
			v.stack(c, o)
		}
	})
	v.evict()
}

func (v *View) stack(c *ui.Context, o Options) {
	v.list.Key = func(i int) any { return v.rowKey(i) }
	v.list.Header = func(i int) bool { return i < len(v.rows) && v.rows[i].kind == rowHeader }
	l := ui.List(c.Key("stack"), &v.list, len(v.rows)).Grow(1).FillWidth().MinHeight(0).Label(diffsLabelPrefix + o.Label).Background(ui.Transparent).TabStop(false)
	l.Rows(func(r ui.ListRow) {
		rw := v.rows[r.Index]
		e := &v.entries[rw.entry]
		switch rw.kind {
		case rowSection:
			e.Section(r.Context)
		case rowHeader:
			v.header(r.Context, e, o)
		default:
			v.body(r.Context, e, o)
		}
	})
	c.OnWindowShortcut(0, ui.KeyF7, func() { v.step(1) })
	c.OnWindowShortcut(ui.Shift, ui.KeyF7, func() { v.step(-1) })
	c.OnWindowShortcut(ui.Alt, ui.KeyF5, func() { v.stepFile(1) })
	c.OnWindowShortcut(ui.Alt|ui.Shift, ui.KeyF5, func() { v.stepFile(-1) })
}

func (v *View) rowKey(i int) any {
	rw := v.rows[i]
	e := &v.entries[rw.entry]
	switch rw.kind {
	case rowHeader:
		return "h:" + e.Key
	case rowBody:
		return "b:" + e.Key
	}
	return "s:" + strconv.Itoa(rw.entry) + e.Key
}

func (v *View) header(c *ui.Context, e *Entry, o Options) {
	f := v.files[e.Key]
	adds, dels := e.Adds, e.Dels
	if adds == CountUnknown && f.state != nil {
		adds, dels = f.state.Stats()
	}
	h := filediff.HeaderOptions{
		Path: e.Path, OldPath: e.OldPath, Status: e.Status, Adds: adds, Dels: dels,
		Open:     v.isOpen(e),
		OnToggle: func() { v.setOpen(e, !v.isOpen(e)) },
		Actions:  func(c *ui.Context) { v.actions(c, e, o) },
	}
	if o.Viewed != nil {
		h.Viewed = o.Viewed.IsViewed(o.Set, e)
		h.OnViewed = func(on bool) {
			o.Viewed.SetViewed(o.Set, e, on)
			v.setOpen(e, !on)
		}
	}
	filediff.Header(c, h)
}

func (v *View) actions(c *ui.Context, e *Entry, o Options) {
	if e.Actions != nil {
		e.Actions(c)
	}
	if o.OpenFile != nil && e.Status != token.GitDeleted {
		action.IconButton(c, icons.File, "Open File", action.IconOptions{ID: "open-file"}).OnClick(func() { o.OpenFile(e.Path) })
	}
	action.IconButton(c, icons.Copy, "Copy Path", action.IconOptions{ID: "copy-path"}).OnClick(func() { c.WriteClipboard(e.Path) })
}

func (v *View) body(c *ui.Context, e *Entry, o Options) {
	f := v.files[e.Key]
	f.used = v.frame
	filediff.Body(c, func() {
		switch {
		case e.Binary || e.Note != "":
			v.note(c, firstNonEmpty(e.Note, binaryNote))
		case e.large() && !f.forced:
			feedback.Empty(c, feedback.EmptyOptions{Title: largeNote, Action: loadDiffLabel, OnAction: func() { f.forced = true }})
		case f.err != nil:
			v.failure(c, e, f, o)
		default:
			v.diff(c, e, f, o)
		}
	})
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func (v *View) note(c *ui.Context, text string) {
	feedback.Empty(c, feedback.EmptyOptions{Title: text})
}

func (v *View) failure(c *ui.Context, e *Entry, f *File, o Options) {
	var note Note
	if errors.As(f.err, &note) {
		v.note(c, string(note))
		return
	}
	feedback.Error(c, "Cannot load the diff of "+e.Path, f.err, func() { f.drop(); v.request(f, o) })
}

func (v *View) diff(c *ui.Context, e *Entry, f *File, o Options) {
	if e.Live != nil && f.ready && e.Live() != f.liveBuf {
		f.drop()
	}
	if !f.ready {
		v.request(f, o)
	}
	if !f.ready {
		ui.Column(c).FillWidth().Height(placeholderHeight(e)).Children(func() { feedback.Loading(c, loadingLabel) })
		return
	}
	if f.state == nil || f.mode != diffmode.Session.Whitespace && e.Patch != nil {
		if v.builds >= buildsPerFrame {
			c.Invalidate()
			ui.Column(c).FillWidth().Height(placeholderHeight(e)).Children(func() { feedback.Loading(c, loadingLabel) })
			return
		}
		if f.engine != nil {
			f.engine.Close()
		}
		f.build(o)
		v.builds++
		v.Stats.Built++
	}
	d, eng := f.state, f.engine
	eng.Refresh()
	if e.Patch == nil && !eng.Ready() {
		ui.Column(c).FillWidth().Height(placeholderHeight(e)).Children(func() { feedback.Loading(c, loadingLabel) })
		return
	}
	if len(d.Hunks()) == 0 && !d.Comparing() {
		if diffmode.Session.Whitespace != git.WhitespaceNone {
			v.note(c, whitespaceNote)
		} else {
			v.note(c, emptyNote)
		}
		return
	}
	d.Layout = editor.DiffLayout(v.layout)
	d.FitContent = true
	if o.Configure != nil {
		o.Configure(d, e.Path)
	}
	if e.Frame != nil {
		e.Frame(d, eng)
	}
	var base []editor.DiffAnnotation
	if e.Annotations != nil {
		base = e.Annotations(d)
	}
	d.Annotations = eng.Annotations(base)
	v.marker(c, e, f)
	editor.DiffView(c.Key("diff:"+e.Key), d).FillWidth().Label(diffLabelPrefix + e.Path)
}

func (v *View) request(f *File, o Options) {
	e := f.entry
	if f.loading || f.ready {
		return
	}
	if e.Patch != nil {
		f.ready = true
		return
	}
	if e.Load == nil {
		return
	}
	f.gen++
	gen := f.gen
	f.loading, f.err = true, nil
	live := (*editor.Buffer)(nil)
	if e.Live != nil {
		live = e.Live()
	}
	f.liveBuf = live
	load, ctx, post := e.Load, o.Ctx, o.Post
	v.Stats.Loads++
	applog.Go(func() {
		oldText, newText, err := load(ctx, live != nil)
		post(func() {
			if gen != f.gen {
				return
			}
			f.loading, f.err = false, err
			if err == nil {
				f.src = &diffmode.Source{Old: oldText, New: newText, Live: live}
				f.ready = true
			}
		})
	})
}

func (v *View) summary() (files, adds, dels int) {
	for i := range v.entries {
		e := &v.entries[i]
		if !e.isFile() {
			continue
		}
		files++
		if e.Adds != CountUnknown {
			adds += e.Adds
			dels += e.Dels
		}
	}
	return files, adds, dels
}
