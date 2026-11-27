package finder

import (
	"bytes"
	"os"
	"unicode/utf8"

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
	st      *editor.State
	key     previewKey
	loading bool
	message string
	gen     int
}

type previewKey struct {
	path       string
	start, end editor.Pos
	ranged     bool
}

func (p *preview) clear() {
	p.gen++
	p.st, p.key, p.loading, p.message = nil, previewKey{}, false, msgNoPreview
}

func (p *preview) show(loc Location) {
	if loc.Path == "" {
		if p.key != (previewKey{}) || p.message == "" {
			p.clear()
		}
		return
	}
	key := previewKey{path: loc.Path, start: loc.Start, end: loc.End, ranged: loc.HasRange}
	if key == p.key {
		return
	}
	if p.key.path == key.path && p.st != nil {
		p.key = key
		p.reveal(loc)
		return
	}
	p.gen++
	gen := p.gen
	p.key, p.st, p.loading, p.message = key, nil, true, msgLoading
	if buf := p.env.Host.Buffer(loc.Path); buf != nil {
		p.build(gen, loc, buf.String(), "")
		return
	}
	go func() {
		text, message := readPreview(loc.Path)
		p.env.Host.Update(func() { p.build(gen, loc, text, message) })
	}()
}

func (p *preview) build(gen int, loc Location, text, message string) {
	if gen != p.gen {
		return
	}
	p.loading = false
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

func readPreview(path string) (text, message string) {
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
