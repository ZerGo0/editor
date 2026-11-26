package workspace

import (
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ZerGo0/editor/internal/backup"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/document"
	"github.com/ZerGo0/editor/internal/fsutil"
	"github.com/ZerGo0/editor/internal/largefile"
	"github.com/ZerGo0/editor/internal/layout"
	"github.com/ZerGo0/editor/internal/viewer"
	"github.com/ZerGo0/editor/internal/workspace/notices"
	"github.com/egoist/mygo/ui/editor"
)

type docEntry struct {
	doc       *document.Document
	err       error
	loading   bool
	autoSaved uint64
	large     *largefile.State
	saving    bool
	kind      viewer.Kind
	stamp     fileStamp
	since     time.Time
}

func (w *Workspace) entry(path string) *docEntry {
	if e, ok := w.docs[path]; ok {
		return e
	}
	e := &docEntry{loading: true, since: time.Now()}
	w.docs[path] = e
	asText := w.viewers.asText[path]
	meta, restore := w.takeBackup(path)
	store := w.hot.store
	w.refreshEditorConfig([]string{path})
	load := func() loaded {
		var l loaded
		if isUntitled(path) {
			l.doc = document.Empty(path)
		} else {
			l.doc, l.kind, l.stamp, l.err = loadFile(path, asText)
		}
		if restore {
			l.backup, _, l.backupErr = store.Read(path)
		}
		return l
	}
	if w.win == nil {
		w.finishLoad(e, path, load(), meta, restore)
		return e
	}
	go func() {
		l := load()
		w.update(func() { w.finishLoad(e, path, l, meta, restore) })
	}()
	return e
}

type loaded struct {
	doc       *document.Document
	kind      viewer.Kind
	stamp     fileStamp
	err       error
	backup    string
	backupErr error
}

func (w *Workspace) finishLoad(e *docEntry, path string, l loaded, meta backup.Meta, restore bool) {
	e.doc, e.kind, e.stamp, e.err, e.loading = l.doc, l.kind, l.stamp, l.err, false
	if restore {
		w.restoreBackup(e, path, l.backup, meta, l.backupErr)
	}
}

func (w *Workspace) ensureEditor(t *layout.Tab) *docEntry {
	e := w.entry(t.Path)
	if t.Editor != nil || e.doc == nil || !e.doc.Editable() {
		return e
	}
	t.Editor = &editor.State{Buffer: e.doc.Buffer, ReadOnly: e.doc.ReadOnly}
	t.Editor.OnGutterClick = func(line int) { w.gutterClick(t, line) }
	w.attachCompletions(t.Editor)
	w.configure(t.Editor)
	defer w.applyDiagnostics(t.Path)
	if r := t.Restore; r != nil && r.Line < e.doc.Buffer.LineCount() {
		t.Editor.GoTo(editor.Pos{Line: r.Line, Col: min(r.Col, len(e.doc.Buffer.Line(r.Line)))})
		if !r.Reveal {
			t.Editor.SetScrollY(r.ScrollY)
		}
		if r.End != nil {
			buf := e.doc.Buffer
			t.Editor.Select(buf.Offset(editor.Pos{Line: r.Line, Col: r.Col}), buf.Offset(*r.End))
		}
	}
	t.Restore = nil
	return e
}

func (w *Workspace) doc(path string) *document.Document {
	if e, ok := w.docs[path]; ok {
		return e.doc
	}
	return nil
}

func (w *Workspace) docOf(t *layout.Tab) *document.Document {
	if t == nil {
		return nil
	}
	return w.doc(t.Path)
}

func (w *Workspace) tabs() []*layout.Tab {
	var out []*layout.Tab
	for _, g := range w.layout.Groups() {
		out = append(out, g.Tabs...)
	}
	return out
}

func (w *Workspace) dropUnused() {
	open := map[string]bool{}
	for _, t := range w.tabs() {
		open[t.Path] = true
	}
	for path, e := range w.docs {
		if !open[path] {
			if e.doc != nil {
				e.doc.Close()
			}
			delete(w.docs, path)
			w.deleteBackup(path)
		}
	}
	w.dropPendingBackups(open)
	w.dropUnusedBrackets(open)
	w.dropUnusedEditorConfig(open)
	w.dropUnusedViews(open)
	w.dropUnusedViewers(open)
	for path := range w.symbols {
		if !open[path] {
			delete(w.symbols, path)
		}
	}
}

func (w *Workspace) modified() []*document.Document {
	var out []*document.Document
	for _, e := range w.docs {
		if e.doc != nil && e.doc.Modified() {
			out = append(out, e.doc)
		}
	}
	slices.SortFunc(out, func(a, b *document.Document) int { return strings.Compare(a.Path, b.Path) })
	return out
}

func (w *Workspace) HasUnsaved() bool {
	return len(w.modified()) > 0
}

func (w *Workspace) save(docs []*document.Document) bool {
	for _, d := range docs {
		if isUntitled(d.Path) {
			continue
		}
		if d.Large != nil {
			w.saveLarge(d)
			continue
		}
		w.prepareSave(d)
		if err := d.Save(); err != nil {
			w.showError("Cannot save "+filepath.Base(d.Path), err)
			return false
		}
		w.recordHistory(d.Path, "")
	}
	return true
}

func (w *Workspace) Save() {
	if t := w.active.ActiveTab(); t != nil {
		if d := w.doc(t.Path); d != nil && (d.Editable() || d.Large != nil) {
			t.Preview = false
			w.saveThen([]*document.Document{d}, nil)
		}
	}
}

func (w *Workspace) SaveAll() {
	w.saveThen(w.modified(), nil)
}

func (w *Workspace) reloadChanged(dir string) {
	for path, e := range w.docs {
		d := e.doc
		if d == nil || !d.Editable() || d.Modified() || !fsutil.Within(dir, path) || !d.ChangedOnDisk() {
			continue
		}
		go func() {
			s, err := d.ReadDisk()
			if err != nil {
				return
			}
			w.update(func() {
				if d.ApplyDisk(s) {
					w.recordHistory(d.Path, externalChangeLabel)
				}
			})
		}()
	}
}

func (w *Workspace) saveLarge(d *document.Document) {
	e := w.docs[d.Path]
	if e == nil || e.saving {
		return
	}
	e.saving = true
	name := filepath.Base(d.Path)
	id := w.notify(notices.Notice{Tone: token.ToneInfo, Title: "Saving " + name + "...", Busy: true})
	go func() {
		err := d.SaveLarge(w.ctx, nil)
		w.update(func() {
			e.saving = false
			w.notes.Close(id)
			if err != nil {
				w.showError("Cannot save "+name, err)
				return
			}
			w.setNotice("Saved " + name)
		})
	}()
}
