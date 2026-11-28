package viewer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/host"
	"github.com/egoist/mygo/ui"
)

const (
	openCommand    = "open"
	revealFlag     = "-R"
	appFlag        = "-a"
	previewApp     = "Preview"
	defaultMonoFam = "monospace"
	defaultMonoPt  = 13
)

type Options struct {
	OpenAsText  func()
	FontFamily  string
	FontSize    float32
	PreviewSize float32
}

type commandRunner func(name string, args ...string) error

func runCommand(name string, args ...string) error {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w: %s", name, err, strings.TrimSpace(string(out)))
	}
	return nil
}

type pane interface {
	body(c *ui.Context)
	controls(c *ui.Context)
	info() string
	reload()
	pause()
	close()
}

type State struct {
	h      host.Host
	path   string
	kind   Kind
	opts   Options
	pane   pane
	focus  ui.Handle
	svc    ui.Services
	run    commandRunner
	closed atomic.Bool
}

func Open(h host.Host, path string, kind Kind, opts Options) (*State, error) {
	if !kind.ReplacesEditor() && !kind.Previewable() {
		return nil, fmt.Errorf("viewer: no viewer for %q (%s)", path, kind)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("viewer: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("viewer: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("viewer: %q is a directory", path)
	}
	s := newState(h, abs, kind, opts)
	s.pane = s.filePane()
	return s, nil
}

func newState(h host.Host, path string, kind Kind, opts Options) *State {
	if opts.FontFamily == "" {
		opts.FontFamily = defaultMonoFam
	}
	if opts.FontSize <= 0 {
		opts.FontSize = defaultMonoPt
	}
	if opts.PreviewSize <= 0 {
		opts.PreviewSize = token.TextBody.Px()
	}
	return &State{h: h, path: path, kind: kind, opts: opts, run: runCommand}
}

func (s *State) filePane() pane {
	switch s.kind {
	case Image, SVG:
		return newImagePane(s, s.kind)
	case Video, Audio:
		return newMediaPane(s)
	case PDF:
		return newPDFPane(s)
	case Markdown:
		return newMarkdownPane(s)
	}
	return newHexPane(s)
}

func (s *State) Handle() *ui.Handle { return &s.focus }

func (s *State) Reload() {
	if !s.closed.Load() {
		s.pane.reload()
	}
}

func (s *State) Pause() {
	if !s.closed.Load() {
		s.pane.pause()
	}
}

func (s *State) Close() {
	if s.closed.Swap(true) {
		return
	}
	s.pane.close()
}

func (s *State) publish(fn func()) {
	s.h.Update(func() {
		if !s.closed.Load() {
			fn()
		}
	})
}

func (s *State) redraw() { s.publish(func() {}) }

func (s *State) system(args ...string) {
	applog.Go(func() {
		if err := s.run(openCommand, args...); err != nil {
			s.publish(func() { s.h.ShowError("Cannot open "+filepath.Base(s.path), err) })
		}
	})
}

func (s *State) reveal()        { s.system(revealFlag, s.path) }
func (s *State) openDefault()   { s.system(s.path) }
func (s *State) openInPreview() { s.system(appFlag, previewApp, s.path) }

func (s *State) openURL(url string) { s.system(url) }

func (s *State) copyPath(c *ui.Context) {
	c.WriteClipboard(s.path)
	s.h.Notify("Copied the path")
}
