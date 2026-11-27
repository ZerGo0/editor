package symbolnav

import (
	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/search/finder"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const msgNoPreview = "no preview available"

type preview struct {
	env     Env
	st      *editor.State
	path    string
	loading bool
	message string
	gen     int
	escape  func()
	handle  ui.Handle
}

func (pv *preview) reset() {
	pv.gen++
	pv.st, pv.path, pv.loading, pv.message = nil, "", false, ""
}

func (pv *preview) show(res Result, f *finder.FileNode, current int, escape func()) {
	pv.gen++
	pv.escape = escape
	if pv.st != nil && pv.path == f.Path {
		pv.loading = false
		pv.reveal(res, f, current)
		return
	}
	gen := pv.gen
	pv.st, pv.path = nil, f.Path
	if buf := pv.env.Host.Buffer(f.Path); buf != nil {
		pv.build(gen, res, f, current, buf.String(), "")
		return
	}
	pv.loading = true
	applog.Go(func() {
		text, message := finder.ReadPreview(f.Path)
		pv.env.Host.Update(func() { pv.build(gen, res, f, current, text, message) })
	})
}

func (pv *preview) build(gen int, res Result, f *finder.FileNode, current int, text, message string) {
	if gen != pv.gen {
		return
	}
	pv.loading = false
	if message != "" {
		pv.st, pv.message = nil, msgNoPreview
		return
	}
	buf := editor.NewBuffer(text)
	buf.SetLanguage(pv.env.Host.Language(f.Path, text))
	st := &editor.State{Buffer: buf, ReadOnly: true}
	pv.env.Host.ConfigureEditor(st, f.Path)
	st.NoFolding, st.NoActiveLine = true, true
	st.OnInput = func(ev ui.InputEvent) bool {
		if ev.Kind != ui.InputKeyDown || ev.Key != ui.KeyEscape {
			return false
		}
		pv.escape()
		return true
	}
	pv.st = st
	pv.reveal(res, f, current)
}

func (pv *preview) reveal(res Result, f *finder.FileNode, current int) {
	st := pv.st
	buf := st.Buffer
	var marks []editor.TextBackground
	for i := range f.Matches {
		loc := res.Location(f, i)
		if loc.Start.Line >= buf.LineCount() || loc.End.Line >= buf.LineCount() {
			continue
		}
		if st.Theme != nil {
			marks = append(marks, editor.TextBackground{Start: buf.Offset(loc.Start), End: buf.Offset(loc.End), Color: st.Theme.Occurrence})
		}
	}
	st.TextBackgrounds = marks
	loc := res.Location(f, current)
	if loc.Start.Line >= buf.LineCount() || loc.End.Line >= buf.LineCount() {
		return
	}
	st.GoTo(loc.Start)
	st.Select(buf.Offset(loc.Start), buf.Offset(loc.End))
}
