package multidiff

import (
	"slices"

	"github.com/ZerGo0/editor/internal/design/filediff"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	lastHunk  = -2
	startEdge = -1
)

type navTarget struct {
	key     string
	hunk    int
	pending bool
	delta   int
}

func (v *View) hunkCount(i int) int {
	f := v.files[v.entries[i].Key]
	if f == nil || f.state == nil || f.engine == nil || f.entry.Patch == nil && !f.engine.Ready() {
		return CountUnknown
	}
	return len(f.state.Hunks())
}

func edgeOf(delta int) int {
	if delta < 0 {
		return lastHunk
	}
	return startEdge
}

func (v *View) cursor(order []int, delta int) (pos, hunk int) {
	if v.nav.key != "" {
		if p := v.position(order, v.nav.key); p >= 0 {
			return p, v.nav.hunk
		}
	}
	if p := slices.Index(order, v.top); p >= 0 {
		if delta < 0 {
			return p, 0
		}
		return p, startEdge
	}
	return 0, startEdge
}

func (v *View) position(order []int, key string) int {
	return slices.IndexFunc(order, func(i int) bool { return v.entries[i].Key == key })
}

func (v *View) step(delta int) {
	order := v.fileIndexes()
	if len(order) == 0 {
		return
	}
	pos, hunk := v.cursor(order, delta)
	v.advance(order, pos, hunk, delta)
}

func (v *View) advance(order []int, pos, hunk, delta int) {
	for pos >= 0 && pos < len(order) {
		i := order[pos]
		n := v.hunkCount(i)
		if n == CountUnknown {
			target := 0
			if delta < 0 {
				target = lastHunk
			}
			v.aim(i, target, delta)
			return
		}
		if hunk == lastHunk {
			hunk = n
		}
		if next := hunk + delta; next >= 0 && next < n {
			v.aim(i, next, delta)
			return
		}
		pos += delta
		hunk = edgeOf(delta)
	}
}

func (v *View) aim(entry, hunk, delta int) {
	e := &v.entries[entry]
	v.nav = navTarget{key: e.Key, hunk: hunk, pending: true, delta: delta}
	if !v.isOpen(e) {
		v.setOpen(e, true)
		v.layoutRows()
	}
	header := v.headerRow[e.Key]
	if body := header + 1; body < len(v.rows) && v.rows[body].entry == entry && v.rows[body].kind == rowBody {
		v.list.ScrollTo(body, ui.Start)
		return
	}
	v.list.ScrollTo(header, ui.Start)
}

func (v *View) stepFile(delta int) {
	order := v.fileIndexes()
	pos := slices.Index(order, v.top)
	if pos < 0 || pos+delta < 0 || pos+delta >= len(order) {
		return
	}
	v.nav.key = ""
	v.Reveal(v.entries[order[pos+delta]].Key)
}

func (v *View) hasFile(delta int) bool {
	pos := slices.Index(v.fileIndexes(), v.top)
	return pos >= 0 && pos+delta >= 0 && pos+delta < len(v.fileIndexes())
}

func (v *View) marker(c *ui.Context, e *Entry, f *File) {
	if !v.nav.pending || v.nav.key != e.Key || f.state == nil {
		return
	}
	hunks := f.state.Hunks()
	if len(hunks) == 0 {
		v.nav.pending = false
		order := v.fileIndexes()
		v.advance(order, v.position(order, e.Key)+v.nav.delta, edgeOf(v.nav.delta), v.nav.delta)
		return
	}
	h := v.nav.hunk
	if h == lastHunk {
		h = len(hunks) - 1
	}
	h = min(max(h, 0), len(hunks)-1)
	v.nav.hunk, v.nav.pending = h, false
	y := f.state.LineTop(editor.DiffNew, hunks[h].NewStart) - float64(filediff.Height())
	ui.Box(c.Key("hunk-marker")).Absolute().Top(float32(max(y, 0))).Left(0).Width(token.BorderWidth.Px()).Height(token.BorderWidth.Px()).AccessHidden(true).
		ScrollIntoView(ui.ScrollIntoViewOptions{Block: ui.ScrollStart})
}
