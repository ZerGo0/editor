package viewer

import (
	"path/filepath"
	"testing"
)

func openMarkdown(t *testing.T, name, text string) (*State, *markdownPane, *fakeHost) {
	t.Helper()
	h := newFakeHost()
	path := writeFile(t, t.TempDir(), name, []byte(text))
	s, err := Open(h, path, Markdown, Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s, s.pane.(*markdownPane), h
}

func TestMarkdownFileViewLoadsAndReloads(t *testing.T) {
	s, p, h := openMarkdown(t, "a.md", "# First")
	tt := viewTester(s)
	if !tt.HasText("Loading Markdown") {
		t.Errorf("texts: %q", tt.Texts())
	}
	settle(t, h, tt, func() bool { return !p.loading })
	if !tt.HasText("First") || !tt.HasText("Markdown") {
		t.Errorf("texts: %q", tt.Texts())
	}
	writeFile(t, filepath.Dir(s.path), "a.md", []byte("# Second"))
	s.Reload()
	settle(t, h, tt, func() bool { return p.source == "# Second" })
	if !tt.HasText("Second") {
		t.Errorf("texts: %q", tt.Texts())
	}
}

func TestMarkdownLinks(t *testing.T) {
	s, p, h := openMarkdown(t, "a.md", "")
	calls := recordCommands(s)
	dir := filepath.Dir(s.path)
	other := writeFile(t, dir, "other file.md", []byte("x"))

	p.link("https://example.com/page")
	if got := nextCommand(t, calls); got[0] != "open" || got[1] != "https://example.com/page" {
		t.Errorf("web link ran %v", got)
	}
	p.link("javascript:alert(1)")
	p.link("file:///etc/passwd")
	p.link("#install")
	if p.opts.Anchor != "#install" || p.opts.AnchorVersion != 1 {
		t.Errorf("anchor %q version %d", p.opts.Anchor, p.opts.AnchorVersion)
	}
	p.link("other%20file.md#section")
	if len(h.opened) != 1 || h.opened[0].path != other {
		t.Fatalf("opened %+v, want %s", h.opened, other)
	}
	p.link("missing.md")
	if len(h.opened) != 1 || h.lastNote() != "Cannot open missing.md: file not found" {
		t.Errorf("opened %+v note %q", h.opened, h.lastNote())
	}
	select {
	case extra := <-calls:
		t.Errorf("an unsafe link ran %v", extra)
	default:
	}
}
