package viewer

import (
	"errors"
	"testing"

	"github.com/ZerGo0/editor/internal/host"
	"github.com/egoist/mygo/ui"
)

type fakeSurface struct {
	loaded   []string
	reloads  int
	closed   bool
	loadErr  error
	finished func()
	failed   func(error)
}

func (f *fakeSurface) show(c *ui.Context) ui.Element { return ui.Box(c) }
func (f *fakeSurface) load(path string) error        { f.loaded = append(f.loaded, path); return f.loadErr }
func (f *fakeSurface) reload()                       { f.reloads++ }
func (f *fakeSurface) close()                        { f.closed = true }
func (f *fakeSurface) onFinished(fn func())          { f.finished = fn }
func (f *fakeSurface) onFailed(fn func(error))       { f.failed = fn }

func openPDF(t *testing.T, open func(host.Host) (pdfSurface, error)) (*State, *pdfPane, *fakeHost, *ui.Tester) {
	t.Helper()
	h := newFakeHost()
	path := writeFile(t, t.TempDir(), "doc.pdf", []byte("%PDF-1.4"))
	s := newState(h, path, PDF, Options{})
	p := &pdfPane{s: s, open: open}
	p.start()
	s.pane = p
	t.Cleanup(s.Close)
	return s, p, h, viewTester(s)
}

func TestPDFLoadsInAWebView(t *testing.T) {
	surface := &fakeSurface{}
	s, p, h, tt := openPDF(t, func(host.Host) (pdfSurface, error) { return surface, nil })
	if len(surface.loaded) != 1 || surface.loaded[0] != s.path || !p.loading {
		t.Fatalf("loaded %v loading %v", surface.loaded, p.loading)
	}
	if _, ok := tt.Access(pdfSpinnerLabel); !ok {
		t.Error("no spinner while the PDF loads")
	}
	surface.finished()
	settle(t, h, tt, func() bool { return !p.loading })
	if _, ok := tt.Access(pdfSpinnerLabel); ok {
		t.Error("the spinner stays after the PDF loaded")
	}
	if !tt.HasText("PDF") || !tt.HasText("Open in Preview") {
		t.Errorf("texts: %q", tt.Texts())
	}
}

func TestPDFLoadFailureOffersRetry(t *testing.T) {
	surface := &fakeSurface{}
	_, p, h, tt := openPDF(t, func(host.Host) (pdfSurface, error) { return surface, nil })
	surface.failed(errors.New("cannot load"))
	settle(t, h, tt, func() bool { return p.err != nil })
	if !tt.HasText("Cannot show this PDF") || !tt.HasText("cannot load") {
		t.Fatalf("texts: %q", tt.Texts())
	}
	if err := tt.Click("Retry"); err != nil {
		t.Fatal(err)
	}
	if len(surface.loaded) != 2 || p.err != nil {
		t.Errorf("loads %d err %v", len(surface.loaded), p.err)
	}
}

func TestPDFFallsBackToPreviewWithoutAWebView(t *testing.T) {
	s, p, _, tt := openPDF(t, func(host.Host) (pdfSurface, error) { return nil, errors.New("no window") })
	if p.unavailable == nil || !tt.HasText("PDF preview is not available") {
		t.Fatalf("texts: %q", tt.Texts())
	}
	calls := recordCommands(s)
	if err := tt.Click("Open in Preview"); err != nil {
		t.Fatal(err)
	}
	if got := nextCommand(t, calls); got[1] != "-a" || got[2] != "Preview" {
		t.Errorf("command %v", got)
	}
}

func TestPDFReloadAndClose(t *testing.T) {
	surface := &fakeSurface{}
	s, _, _, _ := openPDF(t, func(host.Host) (pdfSurface, error) { return surface, nil })
	s.Reload()
	if surface.reloads != 1 {
		t.Errorf("reloads = %d", surface.reloads)
	}
	s.Close()
	if !surface.closed {
		t.Error("Close did not destroy the web view")
	}
}
