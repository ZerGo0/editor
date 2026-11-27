package finder

import (
	"bytes"
	"os"
	"time"
	"unicode/utf8"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/design/picker"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	previewMaxBytes  = 1 << 20
	binarySniffBytes = 8000
	msgNoPreview     = "Select a file or match to preview it"
	msgLoading       = "Loading preview..."
	msgTooLarge      = "The file is too large to preview"
	msgBinary        = "The file is not text"
	msgUnreadable    = "Cannot read the file"
)

type preview struct {
	env     Env
	now     func() time.Time
	st      *editor.State
	shown   string
	title   string
	want    previewKey
	pending bool
	since   time.Time
	message string
	gen     int
}

type previewKey struct {
	path       string
	start, end editor.Pos
	ranged     bool
}

func (p *preview) clock() time.Time {
	if p.now != nil {
		return p.now()
	}
	return time.Now()
}

func (p *preview) clear() {
	p.gen++
	p.st, p.shown, p.title, p.want, p.pending, p.message = nil, "", "", previewKey{}, false, msgNoPreview
}

func (p *preview) busy() bool {
	return p.pending && p.clock().Sub(p.since) >= token.SpinnerDelay
}

func (p *preview) pane(c *ui.Context, layout *picker.Layout, results *picker.ResultsState) {
	busy := p.busy()
	message := p.message
	switch {
	case busy:
		message = msgLoading
	case p.pending:
		c.After(token.SpinnerDelay - p.clock().Sub(p.since))
		if p.st == nil {
			message = ""
		}
	}
	picker.Preview(c, p.st, picker.PreviewOptions{Label: previewLabel, Title: p.title, Message: message, Busy: busy, Layout: layout, Results: results})
}

func (p *preview) show(loc Location) {
	if loc.Path == "" {
		if p.want != (previewKey{}) || p.message == "" {
			p.clear()
		}
		return
	}
	key := previewKey{path: loc.Path, start: loc.Start, end: loc.End, ranged: loc.HasRange}
	if key == p.want {
		return
	}
	p.want = key
	p.gen++
	if p.st != nil && p.shown == loc.Path {
		p.pending = false
		p.reveal(loc)
		return
	}
	gen := p.gen
	p.pending, p.since = true, p.clock()
	if buf := p.env.Host.Buffer(loc.Path); buf != nil {
		p.build(gen, loc, buf.String(), "")
		return
	}
	applog.Go(func() {
		text, message := ReadPreview(loc.Path)
		p.env.Host.Update(func() { p.build(gen, loc, text, message) })
	})
}

func (p *preview) build(gen int, loc Location, text, message string) {
	if gen != p.gen {
		return
	}
	p.pending = false
	p.shown, p.title = loc.Path, p.relative(loc.Path)
	if message != "" {
		p.st, p.message = nil, message
		return
	}
	buf := editor.NewBuffer(text)
	buf.SetLanguage(p.env.Host.Language(loc.Path, text))
	st := &editor.State{Buffer: buf, ReadOnly: true}
	p.env.Host.ConfigureEditor(st, loc.Path)
	st.NoFolding, st.NoActiveLine = true, true
	p.st = st
	p.reveal(loc)
}

func (p *preview) relative(path string) string {
	return relTo(p.env.Host.RootDir(), path)
}

func (p *preview) reveal(loc Location) {
	if p.st == nil || !loc.HasRange {
		return
	}
	buf := p.st.Buffer
	if loc.Start.Line >= buf.LineCount() {
		return
	}
	p.st.GoTo(loc.Start)
	end := loc.End
	if end.Line >= buf.LineCount() {
		end = loc.Start
	}
	p.st.Select(buf.Offset(loc.Start), buf.Offset(end))
}

func ReadPreview(path string) (text, message string) {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return "", msgUnreadable
	}
	if info.Size() > previewMaxBytes {
		return "", msgTooLarge
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", msgUnreadable
	}
	if bytes.IndexByte(data[:min(len(data), binarySniffBytes)], 0) >= 0 || !utf8.Valid(data) {
		return "", msgBinary
	}
	return string(data), ""
}
