package symbols

import (
	"path/filepath"
	"sync"
	"testing"
)

func collect(x *Index) map[string][]string {
	out := map[string][]string{}
	x.All(func(path string, s Symbol) bool {
		out[path] = append(out[path], s.Name)
		return true
	})
	return out
}

func TestIndexUpdateReplaces(t *testing.T) {
	x := NewIndex()
	path := filepath.Join("src", "a.go")
	x.Update(path, langGo, "func One() {}\nfunc Two() {}\n")
	if x.Len() != 2 {
		t.Fatalf("Len = %d, want 2", x.Len())
	}
	x.Update(path, langGo, "func Three() {}\n")
	got := collect(x)
	if x.Len() != 1 || len(got[path]) != 1 || got[path][0] != "Three" {
		t.Errorf("after replace Len=%d got=%v", x.Len(), got)
	}
	x.Update(path, "Unknown", "func Four() {}")
	if x.Len() != 0 {
		t.Errorf("unknown language should clear file, Len = %d", x.Len())
	}
}

func TestIndexRemove(t *testing.T) {
	x := NewIndex()
	x.Update("a.go", langGo, "func A() {}\n")
	x.Update("b.go", langGo, "func B() {}\nfunc C() {}\n")
	x.Remove("b.go")
	x.Remove("missing.go")
	if x.Len() != 1 {
		t.Errorf("Len = %d, want 1", x.Len())
	}
}

func TestIndexRemoveUnder(t *testing.T) {
	x := NewIndex()
	dir := filepath.Join("proj", "pkg")
	x.Update(filepath.Join(dir, "a.go"), langGo, "func A() {}\n")
	x.Update(filepath.Join(dir, "sub", "b.go"), langGo, "func B() {}\n")
	x.Update(filepath.Join("proj", "pkgother", "c.go"), langGo, "func C() {}\n")
	x.RemoveUnder(dir + string(filepath.Separator))
	got := collect(x)
	if len(got) != 1 || x.Len() != 1 {
		t.Errorf("got %v Len %d", got, x.Len())
	}
	x.RemoveUnder("")
	if x.Len() != 1 {
		t.Errorf("empty dir removed files, Len = %d", x.Len())
	}
}

func TestIndexAllStopsEarly(t *testing.T) {
	x := NewIndex()
	x.Update("a.go", langGo, "func A() {}\nfunc B() {}\nfunc C() {}\n")
	calls := 0
	x.All(func(string, Symbol) bool {
		calls++
		return false
	})
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}

func TestIndexConcurrent(t *testing.T) {
	x := NewIndex()
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			path := filepath.Join("f", string(rune('a'+i))+".go")
			for range 50 {
				x.Update(path, langGo, "func A() {}\n")
				x.All(func(string, Symbol) bool { return true })
				x.Remove(path)
			}
		}()
	}
	wg.Wait()
	if x.Len() != 0 {
		t.Errorf("Len = %d, want 0", x.Len())
	}
}
