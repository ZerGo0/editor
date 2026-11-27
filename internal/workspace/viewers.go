package workspace

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/document"
	"github.com/ZerGo0/editor/internal/fsutil"
	"github.com/ZerGo0/editor/internal/layout"
	"github.com/ZerGo0/editor/internal/viewer"
	"github.com/egoist/mygo/ui"
)

const (
	previewPrefix      = "preview:"
	previewTitlePrefix = "Preview "
	previewToSideLabel = "Open Preview to the Side"
	noPreviewNotice    = "Preview is available for Markdown and SVG files."
)

type viewerState struct {
	open   map[*layout.Tab]*viewer.State
	shown  map[*layout.Tab]bool
	drawn  map[*layout.Tab]bool
	asText map[string]bool
}

type fileStamp struct {
	mod  time.Time
	size int64
}

func (s fileStamp) same(o fileStamp) bool {
	return s.size == o.size && s.mod.Equal(o.mod)
}

func newViewerState() viewerState {
	return viewerState{
		open:   map[*layout.Tab]*viewer.State{},
		shown:  map[*layout.Tab]bool{},
		drawn:  map[*layout.Tab]bool{},
		asText: map[string]bool{},
	}
}

func statFile(path string) (fileStamp, error) {
	info, err := os.Stat(path)
	if err != nil {
		return fileStamp{}, err
	}
	return fileStamp{mod: info.ModTime(), size: info.Size()}, nil
}

func stampOrZero(path string) fileStamp {
	stamp, err := statFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("stat %s: %v", path, err)
	}
	return stamp
}

func loadFile(path string, asText bool) (*document.Document, viewer.Kind, fileStamp, error) {
	if asText {
		d, err := document.OpenText(path)
		return d, viewer.None, fileStamp{}, err
	}
	if kind, err := viewer.DetectFile(path); err == nil && kind.ReplacesEditor() {
		stamp, err := statFile(path)
		return nil, kind, stamp, err
	}
	d, err := document.Open(path)
	return d, viewer.None, fileStamp{}, err
}

func (w *Workspace) viewerContent(c *ui.Context, g *layout.Group, tab *layout.Tab, e *docEntry) {
	v := w.viewers.open[tab]
	if v == nil {
		path := tab.Path
		opts := w.viewerOptions()
		opts.OpenAsText = func() { w.openAsText(path) }
		opened, err := viewer.Open(w, path, e.kind, opts)
		if err != nil {
			placeholder(c, "Cannot open this file", err.Error())
			return
		}
		v = opened
		w.viewers.open[tab] = v
	}
	w.viewers.drawn[tab] = true
	v.View(c.Key(tab.ID)).Grow(1).FillWidth()
	w.focusContent(c, g, tab, v.Handle())
}

func (w *Workspace) openAsText(path string) {
	w.viewers.asText[path] = true
	for t := range w.viewers.open {
		if t.Path == path {
			w.dropViewer(t)
		}
	}
	delete(w.docs, path)
}

func (w *Workspace) dropViewer(t *layout.Tab) {
	if v := w.viewers.open[t]; v != nil {
		v.Close()
	}
	delete(w.viewers.open, t)
	delete(w.viewers.shown, t)
	delete(w.viewers.drawn, t)
}

func (w *Workspace) dropUnusedViewers(open map[string]bool) {
	tabs := map[*layout.Tab]bool{}
	for _, t := range w.tabs() {
		tabs[t] = true
	}
	for t := range w.viewers.open {
		if !tabs[t] {
			w.dropViewer(t)
		}
	}
	for p := range w.viewers.asText {
		if !open[p] {
			delete(w.viewers.asText, p)
		}
	}
}

func (w *Workspace) endViewerFrame() {
	vs := &w.viewers
	for t := range vs.shown {
		if v := vs.open[t]; v != nil && !vs.drawn[t] {
			v.Pause()
		}
	}
	vs.shown, vs.drawn = vs.drawn, vs.shown
	clear(vs.drawn)
}

func (w *Workspace) reloadViewers(dir string) {
	for path, e := range w.docs {
		if !e.kind.ReplacesEditor() || !fsutil.Within(dir, path) {
			continue
		}
		old := e.stamp
		applog.Go(func() {
			stamp, err := statFile(path)
			if err != nil || stamp.same(old) {
				return
			}
			w.update(func() {
				e.stamp = stamp
				for t, v := range w.viewers.open {
					if t.Path == path {
						v.Reload()
					}
				}
			})
		})
	}
}

func (w *Workspace) closeViewers() {
	for t := range w.viewers.open {
		w.dropViewer(t)
	}
	for _, v := range w.virtual {
		if v.close != nil {
			v.close()
		}
	}
}

func (w *Workspace) OpenPreview() {
	w.openPreview(w.active)
}

func (w *Workspace) OpenPreviewToSide() {
	w.openPreview(nil)
}

func (w *Workspace) openPreview(g *layout.Group) {
	path := w.ActivePath()
	d := w.doc(path)
	if path == "" || isVirtual(path) || !viewer.Detect(path, nil).Previewable() || d == nil || d.Buffer == nil {
		w.setNotice(noPreviewNotice)
		return
	}
	p := virtualPrefix + previewPrefix + path
	if _, ok := w.virtual[p]; !ok {
		pv := viewer.NewPreview(w, path, d.Buffer.String, w.viewerOptions())
		if pv == nil {
			w.setNotice(noPreviewNotice)
			return
		}
		w.virtual[p] = &virtualTab{title: previewTitlePrefix + filepath.Base(path), view: func(c *ui.Context) { pv.View(c).Grow(1).FillWidth() }, close: pv.Close}
	}
	if g == nil {
		g = w.sideGroup()
	}
	w.openIn(g, p, false, true)
}
