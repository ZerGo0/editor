package viewer

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/design/detail"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
)

const (
	markdownLabel  = "Markdown preview"
	markdownScroll = "markdown-scroll"
)

var webSchemes = []string{"http://", "https://", "mailto:"}

type markdownPane struct {
	s        *State
	opts     markdown.Options
	source   string
	size     int64
	loading  bool
	err      error
	gen      uint64
	fromFile bool
}

func newMarkdownPane(s *State) *markdownPane {
	p := newMarkdownSurface(s)
	p.fromFile = true
	p.load()
	return p
}

func newMarkdownSurface(s *State) *markdownPane {
	p := &markdownPane{s: s}
	p.opts = markdown.Options{
		Theme:     &markdown.Theme{FontSize: s.opts.PreviewSize, CodeFont: s.opts.FontFamily},
		OnLink:    p.link,
		LoadImage: markdown.FileImages(filepath.Dir(s.path)),
	}
	return p
}

func (p *markdownPane) setText(text string) {
	p.source, p.size = text, int64(len(text))
}

func (p *markdownPane) load() {
	p.gen++
	gen := p.gen
	path := p.s.path
	p.loading, p.err = true, nil
	applog.Go(func() {
		data, err := os.ReadFile(path)
		p.s.publish(func() {
			if gen != p.gen {
				return
			}
			p.loading = false
			if err != nil {
				p.err = err
				return
			}
			p.setText(string(data))
		})
	})
	p.s.redraw()
}

func (p *markdownPane) reload() {
	if p.fromFile {
		p.load()
	}
}

func (p *markdownPane) pause() {}

func (p *markdownPane) close() { p.gen++ }

func (p *markdownPane) controls(c *ui.Context) {}

func (p *markdownPane) info() string {
	if p.loading || p.err != nil {
		return ""
	}
	return joinInfo("Markdown", formatSize(p.size))
}

func (p *markdownPane) body(c *ui.Context) {
	switch {
	case p.err != nil:
		detail.Failed(c, "Cannot read this file", p.err, p.load)
	case p.loading && p.source == "":
		detail.Loading(c, "Loading Markdown")
	case strings.TrimSpace(p.source) == "":
		message(c, "Nothing to preview", "Write some Markdown to see it here.", "", nil)
	default:
		ui.Scroll(c.Key(markdownScroll)).Grow(1).FillWidth().Padding(token.PadBarText.Px(), token.PadBarText.Px()).Role(ui.RoleArticle).Label(markdownLabel).Children(func() {
			markdown.View(c, p.source, &p.opts)
		})
	}
}

func (p *markdownPane) link(target string) {
	switch {
	case strings.HasPrefix(target, "#"):
		p.opts.Anchor = target
		p.opts.AnchorVersion++
		p.s.redraw()
	case hasWebScheme(target):
		if ui.SafeToOpen(target) {
			p.s.openURL(target)
		}
	default:
		p.openRelative(target)
	}
}

func hasWebScheme(target string) bool {
	lower := strings.ToLower(target)
	for _, scheme := range webSchemes {
		if strings.HasPrefix(lower, scheme) {
			return true
		}
	}
	return false
}

func (p *markdownPane) openRelative(target string) {
	ref, err := url.Parse(target)
	if err != nil || ref.Scheme != "" || ref.Path == "" {
		return
	}
	path := filepath.FromSlash(ref.Path)
	if !filepath.IsAbs(path) {
		path = filepath.Join(filepath.Dir(p.s.path), path)
	}
	if _, err := os.Stat(path); err != nil {
		p.s.h.Notify("Cannot open " + target + ": file not found")
		return
	}
	p.s.h.OpenAt(path, 0, 0)
}
