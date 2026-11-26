package searchview

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/ZerGo0/editor/internal/fuzzy"
	"github.com/ZerGo0/editor/internal/search/everywhere"
	"github.com/ZerGo0/editor/internal/search/symbols"
)

func (v *View) busy() bool {
	if v.text.searching || v.refreshing > 0 || v.replacing {
		return true
	}
	for _, s := range v.sections {
		if s.loading {
			return true
		}
	}
	return false
}

func (v *View) runQuick() {
	if v.quickCancel != nil {
		v.quickCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	v.quickCancel = cancel
	v.quickGen++
	gen := v.quickGen
	q := strings.TrimSpace(v.st.Query)
	scope := v.scope
	limit := quickLimit
	if scope == ScopeAll {
		limit = allSectionLimit
	}
	want := func(s Scope) bool { return scope == ScopeAll || scope == s }
	for s := ScopeFiles; s < scopeCount; s++ {
		if s != ScopeText && !want(s) {
			v.sections[s] = section{}
		}
	}
	sess := v.session()
	if sess == nil {
		v.rowsDirty = true
		return
	}
	if want(ScopeFiles) {
		v.runFiles(sess, q, limit)
	}
	if want(ScopeActions) {
		rs, total := sess.Actions(q, v.actionLimit(scope, limit))
		v.sections[ScopeActions] = section{items: rs, total: total}
	}
	if want(ScopeSymbols) {
		v.runSymbols(ctx, gen, sess, q, limit)
	}
	if want(ScopeGit) {
		v.async(ctx, gen, ScopeGit, func() ([]everywhere.Result, int) { return sess.Git(ctx, q, limit, scope == ScopeGit) })
	}
	v.rowsDirty = true
}

func (v *View) actionLimit(scope Scope, limit int) int {
	if scope == ScopeActions {
		return actionScopeLimit
	}
	return limit
}

func (v *View) runFiles(sess *everywhere.Session, q string, limit int) {
	fq := parseFileQuery(q)
	name := q
	if fq.hasLine {
		name = fq.path
	}
	if name == "" && fq.hasLine {
		v.sections[ScopeFiles] = section{}
		return
	}
	if name == "" && v.scope == ScopeAll {
		limit = recentLimit
	}
	rs, total := sess.Files(name, limit)
	if fq.hasLine {
		for i := range rs {
			rs[i].Line, rs[i].Col, rs[i].HasPos = fq.line, fq.col, true
		}
	}
	v.sections[ScopeFiles] = section{items: rs, total: total}
}

func (v *View) runSymbols(ctx context.Context, gen int, sess *everywhere.Session, q string, limit int) {
	if !v.editorOnly {
		v.async(ctx, gen, ScopeSymbols, func() ([]everywhere.Result, int) { return sess.Symbols(ctx, q, limit) })
		return
	}
	path, text, lang := v.activeDocument()
	rel := v.relPath(path)
	v.async(ctx, gen, ScopeSymbols, func() ([]everywhere.Result, int) {
		return editorSymbols(q, path, rel, lang, text, limit)
	})
}

func (v *View) activeDocument() (path, text, lang string) {
	path = v.h.ActivePath()
	if path == "" {
		return "", "", ""
	}
	buf := v.h.Buffer(path)
	if buf == nil {
		return path, "", ""
	}
	text = buf.String()
	if l := v.h.Language(path, text); l != nil {
		lang = l.Name
	}
	return path, text, lang
}

func (v *View) relPath(p string) string {
	if r, err := filepath.Rel(v.h.RootDir(), p); err == nil && filepath.IsLocal(r) {
		return filepath.ToSlash(r)
	}
	return p
}

func editorSymbols(q, path, rel, lang, text string, limit int) ([]everywhere.Result, int) {
	syms := symbols.Scan(lang, text)
	var out []everywhere.Result
	for _, s := range syms {
		r := everywhere.SymbolResult(s, nil, path, rel)
		if q != "" {
			m, ok := fuzzy.Find(q, s.Name)
			if !ok {
				continue
			}
			r.Positions = m.Positions
		}
		out = append(out, r)
	}
	return out[:min(len(out), limit)], len(out)
}

func (v *View) async(ctx context.Context, gen int, s Scope, fn func() ([]everywhere.Result, int)) {
	v.sections[s] = section{items: v.sections[s].items, total: v.sections[s].total, loading: true}
	go func() {
		rs, total := fn()
		if ctx.Err() != nil {
			return
		}
		v.post(func() {
			if gen != v.quickGen {
				return
			}
			v.sections[s] = section{items: rs, total: total}
			v.rowsDirty = true
		})
	}()
}
