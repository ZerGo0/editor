package searchview

import (
	"github.com/ZerGo0/editor/internal/host"
	"github.com/ZerGo0/editor/internal/search/everywhere"
	"github.com/ZerGo0/editor/internal/search/files"
	"github.com/ZerGo0/editor/internal/settings"
	"github.com/egoist/mygo/ui/editor"
)

type Location struct {
	Path     string
	Start    editor.Pos
	End      editor.Pos
	HasRange bool
}

type OpenOptions struct {
	Preview bool
	Focus   bool
	Side    bool
}

type Env struct {
	Host        host.Host
	Providers   *everywhere.Search
	Files       *files.Index
	Settings    *settings.Store
	OpenFiles   func() []string
	Open        func(loc Location, opts OpenOptions)
	Reveal      func(path string)
	RunAction   func(id string)
	GitRef      func(ref everywhere.GitRef)
	GitCommit   func(oid string)
	BeforeWrite func(path string)
	FocusEditor func()
}

func (e Env) openFiles() []string {
	if e.OpenFiles == nil {
		return nil
	}
	return e.OpenFiles()
}

func (e Env) open(loc Location, opts OpenOptions) {
	if e.Open != nil {
		e.Open(loc, opts)
		return
	}
	if loc.HasRange {
		e.Host.OpenAt(loc.Path, loc.Start.Line, loc.Start.Col)
		return
	}
	e.Host.OpenAt(loc.Path, -1, 0)
}

func (e Env) excludePatterns() []string {
	if e.Settings == nil {
		return nil
	}
	return append(append([]string(nil), e.Settings.Strings(settings.Exclude)...), e.Settings.Strings(settings.SearchExclude)...)
}

func (e Env) useIgnoreFiles() bool {
	return e.Settings == nil || e.Settings.Bool(settings.SearchUseIgnoreFiles)
}

func (e Env) smartCase() bool {
	return e.Settings != nil && e.Settings.Bool(settings.SearchSmartCase)
}
