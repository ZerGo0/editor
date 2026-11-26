package redline

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAllowlist(t *testing.T) {
	img := canvasOf()
	fill(img, Rect{X: 6, Y: 5, W: 12, H: 12}, testInk)
	res := run(t, img, []Box{{Role: "button", Label: "Refresh", Rect: Rect{X: 0, Y: 0, W: 22, H: 22}}})
	path := filepath.Join(t.TempDir(), "allow.txt")
	if l, err := LoadAllowlist(path); err != nil || len(l.Entries) != 0 {
		t.Fatalf("missing file: %v, %v", l, err)
	}
	body := "te optical button \"Refresh\" -- icon box is wider on purpose\nother optical button \"Refresh\" -- other view\n"
	if err := os.WriteFile(path, []byte(body), allowFilePerm); err != nil {
		t.Fatal(err)
	}
	l, err := LoadAllowlist(path)
	if err != nil {
		t.Fatal(err)
	}
	if bad := l.Check(res); len(bad) != 0 {
		t.Errorf("allowed finding reported: %v", bad)
	}
	if stale := l.Stale(); len(stale) != 1 || stale[0] != "other optical button \"Refresh\" -- other view" {
		t.Errorf("stale = %q", stale)
	}
	if err := l.Prune(); err != nil {
		t.Fatal(err)
	}
	if l, err = LoadAllowlist(path); err != nil || len(l.Entries) != 1 || l.Entries[0].View != "te" {
		t.Errorf("pruned list = %+v, %v", l, err)
	}
	if err := os.WriteFile(path, []byte("view key without reason\n"), allowFilePerm); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadAllowlist(path); err == nil {
		t.Error("entry without a reason accepted")
	}
}
