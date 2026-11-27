package multidiff

import (
	"sort"

	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/git/diffmode"
	"github.com/egoist/mygo/ui"
)

const (
	MaxHeld        = 16
	buildsPerFrame = 2
	minPlaceholder = 2
	maxPlaceholder = 40
)

type rowKind uint8

const (
	rowHeader rowKind = iota
	rowBody
	rowSection
)

type row struct {
	entry int
	kind  rowKind
}

type Stats struct {
	Loads int
	Built int
}

type View struct {
	entries    []Entry
	files      map[string]*File
	open       map[string]bool
	rows       []row
	headerRow  map[string]int
	dirty      bool
	list       ui.ListState
	frame      uint64
	reveal     string
	revealPath string
	top        int
	layout     int
	tree       treeState
	nav        navTarget
	opts       Options
	builds     int
	width      float32
	Stats      Stats
}

func New() *View {
	return &View{files: map[string]*File{}, open: map[string]bool{}, dirty: true, tree: newTree()}
}

func (v *View) Entries() []Entry { return v.entries }

func (v *View) SetEntries(entries []Entry) {
	old := v.files
	v.files = make(map[string]*File, len(entries))
	sortTreeOrder(entries)
	v.entries = entries
	for i := range entries {
		e := &entries[i]
		if !e.isFile() {
			continue
		}
		f := old[e.Key]
		delete(old, e.Key)
		if f != nil && f.entry.Fingerprint != e.Fingerprint {
			f.drop()
			delete(v.open, e.Key)
			f = nil
		}
		if f == nil {
			f = &File{}
		}
		f.entry = e
		v.files[e.Key] = f
	}
	for _, f := range old {
		f.drop()
	}
	v.dirty, v.tree.dirty = true, true
}

func (v *View) Close() {
	for _, f := range v.files {
		f.drop()
	}
}

func (v *View) Reveal(key string) { v.reveal = key }

func (v *View) RevealPath(path string) { v.revealPath = path }

func (v *View) keyOfPath(path string) (string, bool) {
	for i := range v.entries {
		if e := &v.entries[i]; e.isFile() && e.Path == path {
			return e.Key, true
		}
	}
	return "", false
}

func (v *View) Held() int {
	n := 0
	for _, f := range v.files {
		if f.held() {
			n++
		}
	}
	return n
}

func (v *View) Top() string {
	if v.top < 0 || v.top >= len(v.entries) {
		return ""
	}
	return v.entries[v.top].Key
}

func (v *View) File(key string) *File { return v.files[key] }

func (v *View) CurrentPath() (string, bool) {
	if v.top < 0 || v.top >= len(v.entries) || !v.entries[v.top].isFile() {
		return "", false
	}
	return v.entries[v.top].Path, true
}

func (v *View) isOpen(e *Entry) bool {
	if open, ok := v.open[e.Key]; ok {
		return open
	}
	if e.Binary {
		return false
	}
	return !v.viewed(e)
}

func (v *View) viewed(e *Entry) bool {
	return v.opts.Viewed != nil && v.opts.Viewed.IsViewed(v.opts.Set, e)
}

func (v *View) setOpen(e *Entry, open bool) {
	v.open[e.Key] = open
	v.dirty = true
}

func (v *View) setAll(open bool) {
	for i := range v.entries {
		if v.entries[i].isFile() {
			v.open[v.entries[i].Key] = open
		}
	}
	v.dirty = true
}

func (v *View) layoutRows() {
	v.rows = v.rows[:0]
	v.headerRow = make(map[string]int, len(v.entries))
	for i := range v.entries {
		e := &v.entries[i]
		if !e.isFile() {
			v.rows = append(v.rows, row{i, rowSection})
			continue
		}
		v.headerRow[e.Key] = len(v.rows)
		v.rows = append(v.rows, row{i, rowHeader})
		if v.isOpen(e) {
			v.rows = append(v.rows, row{i, rowBody})
		}
	}
	v.dirty = false
}

func (v *View) evict() {
	var held []*File
	for _, f := range v.files {
		if f.held() {
			held = append(held, f)
		}
	}
	if len(held) <= MaxHeld {
		return
	}
	sort.Slice(held, func(i, j int) bool { return held[i].used < held[j].used })
	for _, f := range held[:len(held)-MaxHeld] {
		if f.used != v.frame {
			f.drop()
		}
	}
}

func (v *View) current() *File {
	if v.top < 0 || v.top >= len(v.entries) {
		return nil
	}
	return v.files[v.entries[v.top].Key]
}

func (v *View) engine() *diffmode.Engine {
	if f := v.current(); f != nil {
		return f.engine
	}
	return nil
}

func (v *View) fileIndexes() []int {
	var out []int
	for i := range v.entries {
		if v.entries[i].isFile() {
			out = append(out, i)
		}
	}
	return out
}

func (v *View) updateTop() {
	first, _ := v.list.Visible()
	if first < 0 || first >= len(v.rows) {
		return
	}
	i := first
	for i < len(v.rows) && v.rows[i].kind == rowSection {
		i++
	}
	if i < len(v.rows) {
		v.top = v.rows[i].entry
	}
}

func placeholderHeight(e *Entry) float32 {
	rows := min(max(e.weight(), minPlaceholder), maxPlaceholder)
	return float32(rows) * token.RowHeight.Px()
}

func (v *View) treeShown() bool {
	return v.tree.shown && (v.tree.forced || v.width == 0 || v.width >= treeMinTabWidth())
}

func treeMinTabWidth() float32 { return token.DialogWidth.Px() + token.SidebarMinWidth.Px() }
