package finder

import (
	"context"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/ZerGo0/editor/internal/fuzzy"
	"github.com/ZerGo0/editor/internal/palette"
	"github.com/ZerGo0/editor/internal/search/everywhere"
	"github.com/ZerGo0/editor/internal/search/symbols"
)

type activeDoc struct {
	path  string
	text  string
	lang  string
	lines int
}

type quickInput struct {
	env     Env
	tab     Tab
	query   string
	active  activeDoc
	current string
	recent  []palette.Item
	other   []palette.Item
}

func (d *Dialog) quickInput() quickInput {
	in := quickInput{env: d.env, tab: d.tab, query: strings.TrimSpace(d.st.Query)}
	switch d.tab {
	case TabActions:
		if d.env.Commands != nil {
			in.recent, in.other = d.env.Commands()
		}
	case TabFiles:
		in.active = d.activeDocument()
		if in.query == "" {
			in.current = in.active.path
		}
	}
	return in
}

func (d *Dialog) activeDocument() activeDoc {
	h := d.env.Host
	path := h.ActivePath()
	if path == "" {
		return activeDoc{}
	}
	buf := h.Buffer(path)
	if buf == nil {
		return activeDoc{path: path}
	}
	text := buf.String()
	doc := activeDoc{path: path, text: text, lines: buf.LineCount()}
	if lang := h.Language(path, text); lang != nil {
		doc.lang = lang.Name
	}
	return doc
}

func (in quickInput) compute(ctx context.Context, sess *everywhere.Session) []entry {
	switch in.tab {
	case TabAll:
		return in.all(sess)
	case TabActions:
		return in.actions()
	case TabFiles:
		return in.files(ctx, sess)
	}
	return nil
}

func (in quickInput) all(sess *everywhere.Session) []entry {
	if sess == nil {
		return nil
	}
	var out []entry
	if in.query == "" {
		recents := sess.Recent(recentFetchLimit)
		for _, r := range recents[:min(len(recents), recentLimit)] {
			out = append(out, fileEntry(r, groupRecentFiles))
		}
		if extra := len(recents) - recentLimit; extra > 0 {
			out = append(out, entry{kind: kindMore, group: groupRecentFiles, tab: TabFiles, label: moreLabel(extra, "recent file", "recent files")})
		}
		return out
	}
	files, total := sess.Files(in.query, allSectionLimit)
	for _, r := range files {
		out = append(out, fileEntry(r, groupFiles))
	}
	if extra := total - len(files); extra > 0 {
		out = append(out, entry{kind: kindMore, group: groupFiles, tab: TabFiles, label: moreLabel(extra, "file", "files")})
	}
	actions, total := sess.Actions(in.query, allSectionLimit)
	for _, r := range actions {
		out = append(out, actionEntry(r, groupActions))
	}
	if extra := total - len(actions); extra > 0 {
		out = append(out, entry{kind: kindMore, group: groupActions, tab: TabActions, label: moreLabel(extra, "action", "actions")})
	}
	return out
}

func (in quickInput) actions() []entry {
	recent, other := palette.Filter(in.recent, in.query), palette.Filter(in.other, in.query)
	var out []entry
	add := func(items []palette.Item, group string) {
		for _, it := range items {
			e := entry{kind: kindAction, group: group, label: it.Label, keys: it.Keys, positions: it.Positions, actionID: it.ID}
			out = append(out, e)
		}
	}
	if len(recent) == 0 {
		add(other, "")
		return out
	}
	add(recent, groupRecentlyUsed)
	add(other, groupOtherCommands)
	return out
}

func (in quickInput) files(ctx context.Context, sess *everywhere.Session) []entry {
	q := strings.TrimLeft(in.query, " ")
	switch {
	case strings.HasPrefix(q, prefixHelp):
		return helpEntries()
	case strings.HasPrefix(q, prefixLine):
		return in.line(strings.TrimPrefix(q, prefixLine))
	case strings.HasPrefix(q, prefixEditorSymbols):
		return in.editorSymbols(strings.TrimLeft(strings.TrimPrefix(q, prefixEditorSymbols), " "))
	case strings.HasPrefix(q, prefixSymbols):
		return in.workspaceSymbols(ctx, sess, strings.TrimLeft(strings.TrimPrefix(q, prefixSymbols), " "))
	}
	return in.fileResults(sess, q)
}

func (in quickInput) fileResults(sess *everywhere.Session, q string) []entry {
	if sess == nil {
		return nil
	}
	rs, _ := sess.Files(q, filesLimit+1)
	rs = slices.DeleteFunc(rs, func(r everywhere.Result) bool { return in.current != "" && r.Path == in.current })
	hasRecent := slices.ContainsFunc(rs, func(r everywhere.Result) bool { return r.Recent })
	var out []entry
	for _, wantRecent := range []bool{true, false} {
		group := ""
		switch {
		case hasRecent && wantRecent:
			group = groupRecentlyOpened
		case hasRecent:
			group = groupFileResults
		}
		for _, r := range rs {
			if r.Recent == wantRecent {
				out = append(out, fileEntry(r, group))
			}
		}
	}
	return out[:min(len(out), filesLimit)]
}

func (in quickInput) workspaceSymbols(ctx context.Context, sess *everywhere.Session, q string) []entry {
	if q == "" || sess == nil {
		return nil
	}
	rs, _ := sess.Symbols(ctx, q, symbolsLimit)
	out := make([]entry, len(rs))
	for i, r := range rs {
		out[i] = symbolEntry(r, "")
	}
	return out
}

func (in quickInput) editorSymbols(q string) []entry {
	if in.active.lang == "" || in.active.text == "" {
		return nil
	}
	syms := symbols.Scan(in.active.lang, in.active.text)
	rs := fuzzy.Filter(q, len(syms), func(i int) string { return syms[i].Name })
	out := make([]entry, len(rs))
	for i, r := range rs {
		s := syms[r.Index]
		out[i] = entry{kind: kindSymbol, label: s.Name, detail: s.Container, positions: r.Match.Positions, path: in.active.path, line: s.Line, col: s.Col, hasPos: true}
	}
	return out
}

func (in quickInput) line(q string) []entry {
	if in.active.path == "" || in.active.lines == 0 {
		return nil
	}
	line, col, ok := parseLineCol(q)
	if !ok || line < 1 || line > in.active.lines {
		return []entry{{kind: kindHint, label: "Type a line number between 1 and " + strconv.Itoa(in.active.lines), query: prefixLine}}
	}
	label := "Go to line " + strconv.Itoa(line)
	if col > 0 {
		label += ", column " + strconv.Itoa(col)
	}
	return []entry{{kind: kindLine, label: label, path: in.active.path, line: line - 1, col: max(col-1, 0), hasPos: true}}
}

func parseLineCol(q string) (line, col int, ok bool) {
	lineText, colText, hasCol := strings.Cut(strings.TrimSpace(q), lineColSeparator)
	line, err := strconv.Atoi(strings.TrimSpace(lineText))
	if err != nil {
		return 0, 0, false
	}
	if hasCol {
		if col, err = strconv.Atoi(strings.TrimSpace(colText)); err != nil {
			return 0, 0, false
		}
	}
	return line, col, true
}

var quickHelp = []struct{ prefix, detail string }{
	{prefixActions, "Show and Run Commands"},
	{prefixEditorSymbols, "Go to Symbol in Editor"},
	{prefixSymbols, "Go to Symbol in Workspace"},
	{prefixLine, "Go to Line/Column"},
	{prefixText, "Search for Text in Files"},
}

func helpEntries() []entry {
	out := make([]entry, len(quickHelp))
	for i, h := range quickHelp {
		out[i] = entry{kind: kindHint, label: h.prefix, detail: h.detail, query: h.prefix}
	}
	return out
}

func relTo(root, p string) string {
	if r, err := filepath.Rel(root, p); err == nil && filepath.IsLocal(r) {
		return filepath.ToSlash(r)
	}
	return p
}
