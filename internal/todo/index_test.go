package todo

import (
	"testing"

	"github.com/egoist/mygo/ui/editor"
)

func TestIndexUpdateAndRemove(t *testing.T) {
	x := NewIndex()
	goLang := editor.LanguageNamed("Go")
	x.Update("/p/a.go", goLang, "package a\n// TODO: first\n// FIXME: second\n")
	x.Update("/p/b.go", goLang, "package b\n")
	if got := x.Count(); got != 2 {
		t.Fatalf("count = %d, want 2", got)
	}
	files := x.Files()
	if len(files) != 1 || files[0].Path != "/p/a.go" || files[0].Items[0].Text != "first" {
		t.Fatalf("files = %+v", files)
	}
	x.Update("/p/a.go", goLang, "package a\n// TODO: only\n")
	if got := x.Count(); got != 1 {
		t.Fatalf("count after update = %d, want 1", got)
	}
	x.Remove("/p/a.go")
	if got := x.Count(); got != 0 || len(x.Files()) != 0 {
		t.Fatalf("count after remove = %d", got)
	}
}
