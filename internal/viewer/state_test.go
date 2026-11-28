package viewer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestOpenRejectsUnusableInput(t *testing.T) {
	h := newFakeHost()
	dir := t.TempDir()
	file := writeFile(t, dir, "a.txt", []byte("hello"))
	tests := []struct {
		name string
		path string
		kind Kind
	}{
		{"no viewer for the kind", file, None},
		{"missing file", filepath.Join(dir, "missing.png"), Image},
		{"directory", dir, Hex},
	}
	for _, tt := range tests {
		if s, err := Open(h, tt.path, tt.kind, Options{}); err == nil {
			s.Close()
			t.Errorf("%s: Open succeeded", tt.name)
		}
	}
}

func TestOpenPicksThePaneByKind(t *testing.T) {
	dir := t.TempDir()
	h := newFakeHost()
	tests := []struct {
		name string
		kind Kind
		data []byte
		want string
	}{
		{"a.png", Image, nil, "*viewer.imagePane"},
		{"a.svg", SVG, []byte(svgSample), "*viewer.imagePane"},
		{"a.md", Markdown, []byte("# Hi"), "*viewer.markdownPane"},
		{"a.bin", Hex, []byte{0, 1}, "*viewer.hexPane"},
	}
	for _, tt := range tests {
		path := writeFile(t, dir, tt.name, tt.data)
		s, err := Open(h, path, tt.kind, Options{})
		if err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
		if got := typeName(s.pane); got != tt.want {
			t.Errorf("%s: pane %s, want %s", tt.name, got, tt.want)
		}
		s.Close()
	}
}

func typeName(v any) string { return fmt.Sprintf("%T", v) }

func TestHeaderActions(t *testing.T) {
	s, _, h, tt := openHex(t, binaryData(10), Options{})
	calls := recordCommands(s)
	tests := []struct {
		button string
		want   []string
	}{
		{"Reveal in Finder", []string{"open", "-R", s.path}},
		{"Open With Default App", []string{"open", s.path}},
	}
	for _, tc := range tests {
		if err := tt.Click(tc.button); err != nil {
			t.Fatal(err)
		}
		got := nextCommand(t, calls)
		if strings.Join(got, "|") != strings.Join(tc.want, "|") {
			t.Errorf("%s ran %v, want %v", tc.button, got, tc.want)
		}
	}
	if err := tt.Click("Copy Path"); err != nil {
		t.Fatal(err)
	}
	if tt.Clipboard() != s.path || h.lastNote() != "Copied the path" {
		t.Errorf("clipboard %q note %q", tt.Clipboard(), h.lastNote())
	}
}

func TestCommandFailureIsReported(t *testing.T) {
	s, _, h, tt := openHex(t, binaryData(10), Options{})
	s.run = func(string, ...string) error { return os.ErrPermission }
	if err := tt.Click("Reveal in Finder"); err != nil {
		t.Fatal(err)
	}
	settle(t, h, tt, func() bool { return h.lastNote() != "" })
	if !strings.Contains(h.lastNote(), "permission denied") {
		t.Errorf("note %q", h.lastNote())
	}
}

func TestHeaderStaysAccessibleInNarrowViews(t *testing.T) {
	_, _, _, tt := openHex(t, binaryData(10), Options{})
	tt.SetSize(240, 300)
	tt.Frame()
	if a, ok := tt.Access(toolbarLabel); !ok || a.Role != ui.RoleToolbar {
		t.Errorf("toolbar: %+v %v", a, ok)
	}
}

func TestEveryButtonHasAnAccessibleName(t *testing.T) {
	_, p, _, tt := openHex(t, binaryData(64), Options{})
	p.selectAll()
	tt.Frame()
	for _, a := range tt.AccessAll() {
		if a.Role == ui.RoleButton && a.Name == "" {
			t.Errorf("a button at %v has no name", a.Bounds)
		}
	}
}

func TestResultsAfterCloseAreIgnored(t *testing.T) {
	s, p, h, _ := openHex(t, binaryData(10), Options{})
	s.Reload()
	s.Close()
	s.Reload()
	s.Pause()
	h.drain()
	if p.file != nil {
		t.Error("the file stayed open after Close")
	}
}
