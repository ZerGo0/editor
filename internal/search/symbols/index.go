package symbols

import (
	"os"
	"strings"
	"sync"
)

type Index struct {
	mu    sync.RWMutex
	files map[string][]Symbol
	count int
}

type fileSymbols struct {
	path    string
	symbols []Symbol
}

func NewIndex() *Index {
	return &Index{files: make(map[string][]Symbol)}
}

func (x *Index) Update(path, language, text string) {
	symbols := Scan(language, text)
	x.mu.Lock()
	defer x.mu.Unlock()
	x.count += len(symbols) - len(x.files[path])
	if len(symbols) == 0 {
		delete(x.files, path)
		return
	}
	x.files[path] = symbols
}

func (x *Index) Remove(path string) {
	x.mu.Lock()
	defer x.mu.Unlock()
	x.removeLocked(path)
}

func (x *Index) RemoveUnder(dir string) {
	if dir == "" {
		return
	}
	separator := string(os.PathSeparator)
	prefix := strings.TrimRight(dir, separator) + separator
	x.mu.Lock()
	defer x.mu.Unlock()
	for path := range x.files {
		if path == dir || strings.HasPrefix(path, prefix) {
			x.removeLocked(path)
		}
	}
}

func (x *Index) Len() int {
	x.mu.RLock()
	defer x.mu.RUnlock()
	return x.count
}

func (x *Index) All(fn func(path string, s Symbol) bool) {
	x.mu.RLock()
	snapshot := make([]fileSymbols, 0, len(x.files))
	for path, symbols := range x.files {
		snapshot = append(snapshot, fileSymbols{path, symbols})
	}
	x.mu.RUnlock()
	for _, f := range snapshot {
		for _, s := range f.symbols {
			if !fn(f.path, s) {
				return
			}
		}
	}
}

func (x *Index) removeLocked(path string) {
	x.count -= len(x.files[path])
	delete(x.files, path)
}
