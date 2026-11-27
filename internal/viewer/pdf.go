package viewer

import (
	"errors"
	"fmt"
	"os"

	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/detail"
	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/host"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const (
	pdfLabel          = "PDF document"
	pdfSpinnerLabel   = "Loading PDF"
	pdfUnavailableMsg = "The PDF could not be shown inside the editor. Open it in Preview instead."
)

type pdfSurface interface {
	show(c *ui.Context) ui.Element
	load(path string) error
	reload()
	close()
	onFinished(fn func())
	onFailed(fn func(error))
}

type webSurface struct{ view *mygo.WebView }

func newWebSurface(h host.Host) (pdfSurface, error) {
	win := h.Window()
	if win == nil {
		return nil, errors.New("no window to host a web view")
	}
	view, err := win.NewWebView(mygo.WebViewOptions{})
	if err != nil {
		return nil, err
	}
	return &webSurface{view: view}, nil
}

func (w *webSurface) show(c *ui.Context) ui.Element { return ui.WebView(c.Key("pdf"), w.view) }

func (w *webSurface) load(path string) error { return w.view.Page().LoadFile(path) }

func (w *webSurface) reload() { w.view.Page().Reload() }

func (w *webSurface) close() { w.view.Destroy() }

func (w *webSurface) onFinished(fn func()) { w.view.Page().OnDidFinishLoad(fn) }

func (w *webSurface) onFailed(fn func(error)) {
	w.view.Page().OnDidFailLoad(func(e *mygo.LoadError) { fn(e) })
}

type pdfPane struct {
	s           *State
	open        func(h host.Host) (pdfSurface, error)
	web         pdfSurface
	unavailable error
	err         error
	loading     bool
	size        int64
}

func newPDFPane(s *State) *pdfPane {
	p := &pdfPane{s: s, open: newWebSurface}
	p.start()
	return p
}

func (p *pdfPane) start() {
	p.err = nil
	if info, err := os.Stat(p.s.path); err == nil {
		p.size = info.Size()
	}
	if p.web == nil {
		web, err := p.open(p.s.h)
		if err != nil {
			p.unavailable = fmt.Errorf("web view: %w", err)
			return
		}
		web.onFinished(func() { p.s.publish(func() { p.loading = false }) })
		web.onFailed(func(err error) {
			p.s.publish(func() { p.loading, p.err = false, err })
		})
		p.web = web
	}
	p.loading = true
	if err := p.web.load(p.s.path); err != nil {
		p.loading, p.err = false, err
	}
}

func (p *pdfPane) reload() {
	p.err = nil
	if p.web == nil {
		p.start()
		return
	}
	p.loading = true
	p.web.reload()
	p.s.redraw()
}

func (p *pdfPane) pause() {}

func (p *pdfPane) close() {
	if p.web != nil {
		p.web.close()
		p.web = nil
	}
}

func (p *pdfPane) info() string { return joinInfo(formatSize(p.size), "PDF") }

func (p *pdfPane) controls(c *ui.Context) {
	if action.Button(c, "Open in Preview", action.ButtonOptions{Variant: action.Ghost, Small: true, ID: "open-preview"}).Clicked() {
		p.s.openInPreview()
	}
}

func (p *pdfPane) body(c *ui.Context) {
	switch {
	case p.unavailable != nil:
		message(c, "PDF preview is not available", p.unavailable.Error()+". "+pdfUnavailableMsg, "", nil)
	case p.err != nil:
		detail.Failed(c, "Cannot show this PDF", p.err, p.start)
	default:
		view := p.web.show(c).Grow(1).FillWidth().Label(pdfLabel).Bind(&p.s.focus)
		if p.loading {
			view.Children(func() {
				feedback.Spinner(c, pdfSpinnerLabel).Absolute().Top(token.GapControl.Px()).Right(token.GapControl.Px())
			})
		}
	}
}
