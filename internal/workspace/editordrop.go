package workspace

import (
	"os"
	"slices"

	"github.com/ZerGo0/editor/internal/design/chrome"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/layout"
	"github.com/ZerGo0/editor/internal/workspace/filetree"
	"github.com/egoist/mygo/ui"
)

const (
	dropEdgeShare  = 0.1
	dropSideThirds = 3
	dropSplitShare = 50
)

type dropPlace struct {
	split bool
	side  layout.Direction
}

type dropRequest struct {
	group   *layout.Group
	at      int
	place   dropPlace
	copying bool
}

func dropPlaceAt(x, y, w, h float32) dropPlace {
	if w <= 0 || h <= 0 {
		return dropPlace{}
	}
	edgeX, edgeY := w*dropEdgeShare, h*dropEdgeShare
	if x > edgeX && x < w-edgeX && y > edgeY && y < h-edgeY {
		return dropPlace{}
	}
	switch third := w / dropSideThirds; {
	case x < third:
		return dropPlace{true, layout.Left}
	case x > third*(dropSideThirds-1):
		return dropPlace{true, layout.Right}
	case y < h/2:
		return dropPlace{true, layout.Up}
	}
	return dropPlace{true, layout.Down}
}

func dropOverlay(c *ui.Context, p dropPlace) {
	box := ui.Box(c).Absolute().Background(token.Of(c).List.Drop).PassThrough()
	if !p.split {
		box.Top(0).Left(0).Right(0).Bottom(0)
		return
	}
	switch p.side {
	case layout.Left:
		box.Top(0).Left(0).Bottom(0).WidthPercent(dropSplitShare)
	case layout.Right:
		box.Top(0).Right(0).Bottom(0).WidthPercent(dropSplitShare)
	case layout.Up:
		box.Top(0).Left(0).Right(0).HeightPercent(dropSplitShare)
	default:
		box.Left(0).Right(0).Bottom(0).HeightPercent(dropSplitShare)
	}
}

func copyHeld(c *ui.Context) bool {
	return c.Modifiers()&ui.Alt != 0
}

func (w *Workspace) areaDrop(c *ui.Context, area ui.Element, g *layout.Group) {
	copying := copyHeld(c)
	place := func(v any) (dropPlace, bool) {
		if d, ok := v.(tabDrag); ok && d.group == g && len(g.Tabs) == 1 && !copying {
			return dropPlace{}, false
		}
		x, y, _ := area.PointerPosition()
		b := area.Bounds()
		return dropPlaceAt(x, y, b.W, b.H), true
	}
	if v := draggedOver(area); v != nil {
		if p, ok := place(v); ok {
			dropOverlay(c, p)
		}
	}
	if v := droppedValue(area); v != nil {
		if p, ok := place(v); ok {
			w.queueDrop(v, dropRequest{group: g, at: layout.AfterActive, place: p, copying: copying})
		}
	}
	if files := area.DroppedFiles(); len(files) > 0 {
		w.dropFiles(files, dropRequest{group: g, at: layout.AfterActive})
	}
}

func (w *Workspace) tabStripDrop(c *ui.Context, el ui.Element, g *layout.Group, at int) chrome.DropMark {
	over, dropped, files := draggedOver(el), droppedValue(el), el.DroppedFiles()
	if over == nil && dropped == nil && len(files) == 0 {
		return chrome.DropNone
	}
	mark := chrome.DropBefore
	if at < len(g.Tabs) && chrome.DropSide(el) == chrome.DropAfter {
		at++
		mark = chrome.DropAfter
	}
	req := dropRequest{group: g, at: at, copying: copyHeld(c)}
	if dropped != nil {
		w.queueDrop(dropped, req)
	}
	if len(files) > 0 {
		w.dropFiles(files, req)
	}
	if over == nil {
		return chrome.DropNone
	}
	return mark
}

func (w *Workspace) dropFiles(files []string, r dropRequest) {
	for _, p := range files {
		if info, err := os.Stat(p); err == nil && info.IsDir() && w.env.OpenFolder != nil {
			w.env.OpenFolder(w.win, p, false)
		}
	}
	w.queueDrop(filetree.Paths(files), r)
}

func (w *Workspace) queueDrop(v any, r dropRequest) {
	w.update(func() { w.drop(v, r) })
}

func (w *Workspace) drop(v any, r dropRequest) {
	groups := w.layout.Groups()
	if !slices.Contains(groups, r.group) {
		return
	}
	to := r.group
	switch v := v.(type) {
	case tabDrag:
		if !slices.Contains(groups, v.group) || v.group.Index(v.tab) < 0 {
			return
		}
		if r.place.split && v.group == r.group && len(r.group.Tabs) == 1 && !r.copying {
			return
		}
		if r.place.split {
			if to = w.addGroup(r.group, r.place.side); to == nil {
				return
			}
		}
		w.moveTab(v, to, r.at, r.copying)
	case filetree.Paths:
		files := slices.DeleteFunc(slices.Clone(v), isDir)
		if len(files) == 0 {
			return
		}
		if r.place.split {
			if to = w.addGroup(r.group, r.place.side); to == nil {
				return
			}
		}
		w.openDropped(to, files)
	}
}

func isDir(p string) bool {
	info, err := os.Stat(p)
	return err != nil || info.IsDir()
}

func (w *Workspace) openDropped(g *layout.Group, files []string) {
	w.active = g
	for _, p := range files {
		w.OpenFile(p, false, true)
	}
}

func droppedValue(el ui.Element) any {
	if v, ok := ui.Drop[any](el); ok && isWorkbenchDrag(v) {
		return v
	}
	return nil
}

func draggedOver(el ui.Element) any {
	if v, ok := ui.DragOver[any](el); ok && isWorkbenchDrag(v) {
		return v
	}
	return nil
}

func isWorkbenchDrag(v any) bool {
	switch v.(type) {
	case tabDrag, filetree.Paths:
		return true
	}
	return false
}
