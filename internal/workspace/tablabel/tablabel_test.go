package tablabel

import (
	"maps"
	"testing"
)

func TestDescribe(t *testing.T) {
	got := Describe([]string{"/r/a/x/view.go", "/r/b/x/view.go", "/r/a/main.go", "/r/c/y/view.go"})
	want := map[string]string{"/r/a/x/view.go": "…/a/x", "/r/b/x/view.go": "…/b/x", "/r/c/y/view.go": "…/c/y"}
	if !maps.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestUniqueNamesHaveNoDescription(t *testing.T) {
	if got := Describe([]string{"/r/a.go", "/r/b.go", "/r/a.go"}); len(got) != 0 {
		t.Fatalf("got %v", got)
	}
}

func TestShortParentPaths(t *testing.T) {
	got := Describe([]string{"/a/f.go", "/b/f.go"})
	if got["/a/f.go"] != "a" || got["/b/f.go"] != "b" {
		t.Fatalf("got %v", got)
	}
}
