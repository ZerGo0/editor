package multidiff

import (
	"github.com/ZerGo0/editor/internal/findwidget"
	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/git/diffmode"
	"github.com/egoist/mygo/ui/editor"
)

type File struct {
	entry   *Entry
	loading bool
	ready   bool
	err     error
	gen     int
	src     *diffmode.Source
	liveBuf *editor.Buffer
	state   *editor.DiffState
	engine  *diffmode.Engine
	mode    git.WhitespaceMode
	used    uint64
	forced  bool
}

func (f *File) State() *editor.DiffState { return f.state }

func (f *File) Err() error { return f.err }

func (f *File) Settled() bool { return f.state != nil || f.err != nil }

func (f *File) held() bool { return f.state != nil || f.src != nil }

func (f *File) drop() {
	if f.engine != nil {
		f.engine.Close()
	}
	f.gen++
	f.state, f.engine, f.src, f.liveBuf = nil, nil, nil, nil
	f.loading, f.ready, f.err = false, false, nil
}

func (f *File) build(o Options) {
	e := f.entry
	d := &editor.DiffState{RenderFindBar: findwidget.Render, RenderSeparator: separator, FitContent: true}
	f.mode = diffmode.Session.Whitespace
	if e.Patch != nil {
		fp := diffmode.FilterPatch(*e.Patch, f.mode)
		d.SetPatch(fp)
		if d.New != nil && o.Language != nil {
			lang := o.Language(e.Path, d.New.String())
			d.New.SetLanguage(lang)
			if d.Old != nil {
				d.Old.SetLanguage(lang)
			}
		}
		f.engine = diffmode.NewEngine(o.Ctx, d, o.Post)
		f.engine.LineNumber = diffmode.PatchLineNumbers(fp)
		f.engine.Install(nil)
	} else {
		f.engine = diffmode.NewEngine(o.Ctx, d, o.Post)
		f.engine.Install(nil)
		src := *f.src
		if o.Language != nil {
			src.Language = o.Language(e.Path, src.New)
		}
		f.engine.SetSource(src)
	}
	f.state = d
	if e.Setup != nil {
		e.Setup(d, f.engine)
	}
}
