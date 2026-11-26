package todo

import (
	"slices"
	"strings"
	"sync"

	"github.com/egoist/mygo/ui/editor"
)

type File struct {
	Path  string
	Items []Item
}

type Index struct {
	mu    sync.RWMutex
	items map[string][]Item
	count int
	gen   uint64
}

func NewIndex() *Index {
	return &Index{items: map[string][]Item{}}
}

func (x *Index) Update(path string, lang *editor.Language, text string) {
	items := Scan(lang, text, DefaultPatterns)
	x.mu.Lock()
	defer x.mu.Unlock()
	x.count += len(items) - len(x.items[path])
	if len(items) == 0 {
		delete(x.items, path)
	} else {
		x.items[path] = items
	}
	x.gen++
}

func (x *Index) Remove(path string) {
	x.mu.Lock()
	defer x.mu.Unlock()
	x.count -= len(x.items[path])
	delete(x.items, path)
	x.gen++
}

func (x *Index) Count() int {
	x.mu.RLock()
	defer x.mu.RUnlock()
	return x.count
}

func (x *Index) Generation() uint64 {
	x.mu.RLock()
	defer x.mu.RUnlock()
	return x.gen
}

func (x *Index) Files() []File {
	x.mu.RLock()
	out := make([]File, 0, len(x.items))
	for p, items := range x.items {
		out = append(out, File{Path: p, Items: items})
	}
	x.mu.RUnlock()
	slices.SortFunc(out, func(a, b File) int { return strings.Compare(a.Path, b.Path) })
	return out
}
