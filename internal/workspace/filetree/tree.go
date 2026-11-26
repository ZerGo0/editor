package filetree

import (
	"cmp"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ZerGo0/editor/internal/fsutil"
	"github.com/egoist/mygo/ui"
)

const newEntryName = "\x00new"

type Tree struct {
	Root      string
	Hidden    func(path string) bool
	state     ui.OutlineState[string]
	dirs      map[string][]string
	isDir     map[string]bool
	row       int
	pending   string
	focus     bool
	ignored   Ignored
	clip      []string
	cut       bool
	edit      *edit
	slot      ui.Rect
	slotBuilt bool
	chainTop  map[string]string
	git       gitState
}

type edit struct {
	path        string
	create      bool
	folder      bool
	value       string
	err         string
	focused     bool
	selected    bool
	returnFocus bool
}

func New(root string, expanded []string) *Tree {
	t := &Tree{Root: root, dirs: map[string][]string{}, isDir: map[string]bool{root: true}, row: -1, chainTop: map[string]string{}}
	t.state.Generation = 1
	t.state.List.Selected = &t.row
	for _, p := range expanded {
		t.state.Open.Add(p)
		t.isDir[p] = true
	}
	return t
}

func (t *Tree) Expanded() []string {
	return slices.Sorted(t.state.Open.All())
}

func (t *Tree) IsDir(p string) bool {
	return t.isDir[p]
}

func (t *Tree) children(p string) []string {
	if !t.isDir[p] {
		return nil
	}
	if kids, ok := t.dirs[p]; ok {
		return kids
	}
	kids := t.read(p)
	t.dirs[p] = kids
	return kids
}

func (t *Tree) rows(p string) []string {
	kids := t.children(p)
	if e := t.edit; e != nil && e.create && e.path == p {
		return append([]string{t.placeholder()}, kids...)
	}
	return kids
}

func (t *Tree) visibleRows(p string) []string {
	rows := t.rows(p)
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = t.compact(r)
	}
	return out
}

func (t *Tree) compact(p string) string {
	top := p
	for t.isDir[p] && (t.edit == nil || !t.edit.create || t.edit.path != p) {
		kids := t.children(p)
		if len(kids) != 1 || !t.isDir[kids[0]] {
			break
		}
		p = kids[0]
	}
	if p == top {
		delete(t.chainTop, p)
		return p
	}
	t.chainTop[p] = top
	return p
}

func (t *Tree) name(p string) string {
	top, ok := t.chainTop[p]
	if !ok {
		return filepath.Base(p)
	}
	rel, err := filepath.Rel(filepath.Dir(top), p)
	if err != nil {
		return filepath.Base(p)
	}
	return filepath.ToSlash(rel)
}

func (t *Tree) placeholder() string {
	return filepath.Join(t.edit.path, newEntryName)
}

func (t *Tree) isPlaceholder(p string) bool {
	return t.edit != nil && t.edit.create && p == t.placeholder()
}

func (t *Tree) read(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []string{}
	}
	type item struct {
		path string
		dir  bool
	}
	items := make([]item, 0, len(entries))
	for _, e := range entries {
		p := filepath.Join(dir, e.Name())
		if t.Hidden != nil && t.Hidden(p) {
			continue
		}
		isDir := e.IsDir()
		if e.Type()&os.ModeSymlink != 0 {
			if info, err := os.Stat(p); err == nil {
				isDir = info.IsDir()
			}
		}
		t.isDir[p] = isDir
		items = append(items, item{p, isDir})
	}
	slices.SortFunc(items, func(a, b item) int {
		if a.dir != b.dir {
			if a.dir {
				return -1
			}
			return 1
		}
		an, bn := filepath.Base(a.path), filepath.Base(b.path)
		return cmp.Or(strings.Compare(strings.ToLower(an), strings.ToLower(bn)), strings.Compare(an, bn))
	})
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.path
	}
	return out
}

func (t *Tree) Invalidate(dir string) {
	for p := range t.dirs {
		if fsutil.Within(dir, p) {
			delete(t.dirs, p)
		}
	}
	t.state.Generation++
}

func (t *Tree) InvalidateAll() {
	clear(t.dirs)
	t.state.Generation++
}

func (t *Tree) Selected() string {
	if t.row < 0 || t.row >= t.state.Rows() {
		return ""
	}
	if p := t.state.Item(t.row); !t.isPlaceholder(p) {
		return p
	}
	return ""
}

func (t *Tree) Reveal(p string) {
	for dir := filepath.Dir(p); fsutil.Within(t.Root, dir) && dir != t.Root; dir = filepath.Dir(dir) {
		t.state.Open.Add(dir)
		t.isDir[dir] = true
	}
	t.state.Generation++
	t.pending = p
}

func (t *Tree) Expand(dir string) {
	if dir == t.Root || !fsutil.Within(t.Root, dir) {
		return
	}
	t.Reveal(dir)
	t.state.Open.Add(dir)
	t.isDir[dir] = true
}

func (t *Tree) CollapseAll() {
	t.state.Open.Clear()
	t.state.Generation++
}

func (t *Tree) SetIgnored(s Ignored) {
	t.ignored = s
}

func (t *Tree) IsIgnored(p string) bool {
	rel, err := filepath.Rel(t.Root, p)
	if err != nil || !filepath.IsLocal(rel) {
		return false
	}
	return t.ignored.Has(filepath.ToSlash(rel))
}

func (t *Tree) SetClipboard(paths []string, cut bool) {
	t.clip, t.cut = slices.Clone(paths), cut
}

func (t *Tree) Clipboard() ([]string, bool) {
	return t.clip, t.cut
}

func (t *Tree) ClearClipboard() {
	t.clip, t.cut = nil, false
}

func (t *Tree) isCut(p string) bool {
	return t.cut && slices.Contains(t.clip, p)
}

func (t *Tree) StartRename(p string) {
	if p == "" || p == t.Root {
		return
	}
	t.CancelEdit()
	t.edit = &edit{path: p, value: filepath.Base(p)}
	t.pending = p
}

func (t *Tree) StartCreate(dir string, folder, returnFocus bool) {
	t.CancelEdit()
	if dir != t.Root {
		t.Expand(dir)
	}
	t.isDir[dir] = true
	t.edit = &edit{path: dir, create: true, folder: folder, returnFocus: returnFocus}
	t.state.Generation++
	t.pending = t.placeholder()
}

func (t *Tree) Editing() bool {
	return t.edit != nil
}

func (t *Tree) CancelEdit() {
	if t.edit == nil {
		return
	}
	if t.edit.create {
		t.state.Generation++
	}
	t.edit = nil
	t.focus = true
}
