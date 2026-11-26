package everywhere

import (
	"path/filepath"

	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/palette"
	"github.com/ZerGo0/editor/internal/search/files"
	"github.com/ZerGo0/editor/internal/search/symbols"
	"github.com/egoist/mygo/ui/editor"
)

type Env struct {
	Root     string
	Files    *files.Index
	Symbols  *symbols.Index
	Actions  func() []palette.Item
	Recent   func() []string
	Modified func() map[string]string
	Repo     func() *git.Repo
}

type Search struct {
	env *Env
}

func New(env *Env) *Search {
	return &Search{env: env}
}

func (s *Session) rel(p string) string {
	if r, err := filepath.Rel(s.env.Root, p); err == nil && filepath.IsLocal(r) {
		return filepath.ToSlash(r)
	}
	return p
}

type SymbolVisitor struct {
	Index *symbols.Index
}

func (v SymbolVisitor) Update(path string, lang *editor.Language, text string) {
	if symbols.Supports(lang.Name) {
		v.Index.Update(path, lang.Name, text)
		return
	}
	v.Index.Remove(path)
}

func (v SymbolVisitor) Remove(path string) {
	v.Index.Remove(path)
}
