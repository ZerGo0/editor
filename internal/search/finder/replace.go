package finder

import (
	"errors"
	"fmt"

	"github.com/ZerGo0/editor/internal/search/textsearch"
	"github.com/egoist/mygo/ui/editor"
)

type replaceTarget struct {
	file   *FileNode
	accept func(textsearch.Match) bool
}

type matchPos [4]int

func posOf(m textsearch.Match) matchPos {
	return matchPos{m.Start.Line, m.Start.Col, m.End.Line, m.End.Col}
}

func acceptSet(matches []textsearch.Match) func(textsearch.Match) bool {
	set := make(map[matchPos]bool, len(matches))
	for _, m := range matches {
		set[posOf(m)] = true
	}
	return func(m textsearch.Match) bool { return set[posOf(m)] }
}

func (r *ReplacePopup) canReplace() bool {
	return r.st.ShowReplace && r.text.matcher != nil && r.text.err == "" && r.text.total > 0 && !r.replacing && !r.text.searching
}

func (r *ReplacePopup) replacement() textsearch.Replacement {
	return r.text.matcher.Replacement(r.st.With, r.st.PreserveCase)
}

func (r *ReplacePopup) replaceSelected() {
	row, ok := r.current()
	if !ok || !r.canReplace() {
		return
	}
	accept := acceptSet(row.file.Matches)
	if row.match >= 0 {
		accept = acceptSet(row.file.Matches[row.match : row.match+1])
	}
	r.apply([]replaceTarget{{file: row.file, accept: accept}})
}

func (r *ReplacePopup) replaceAll() {
	if !r.canReplace() {
		return
	}
	targets := make([]replaceTarget, len(r.text.files))
	for i, f := range r.text.files {
		targets[i] = replaceTarget{file: f, accept: acceptSet(f.Matches)}
	}
	r.apply(targets)
}

func (r *ReplacePopup) apply(targets []replaceTarget) {
	m, repl := r.text.matcher, r.replacement()
	h := r.env.Host
	r.replacing = true
	var disk []replaceTarget
	replaced, changed := 0, 0
	for _, t := range targets {
		buf := h.Buffer(t.file.Path)
		if buf == nil {
			disk = append(disk, t)
			continue
		}
		plan := m.PlanReplace(buf.String(), repl, t.accept)
		if len(plan.Edits) == 0 {
			continue
		}
		edits := make([]editor.Edit, len(plan.Edits))
		for i, e := range plan.Edits {
			edits[i] = editor.Edit{Start: e.Start, End: e.End, Text: e.Text}
		}
		buf.Edit(edits...)
		replaced += len(edits)
		changed++
	}
	go func() {
		var errs []error
		for _, t := range disk {
			path := t.file.Path
			n, err := textsearch.ReplaceFile(path, m, repl, t.accept, 0, func() { r.beforeWrite(path) })
			if err != nil {
				errs = append(errs, err)
				continue
			}
			if n > 0 {
				replaced += n
				changed++
			}
		}
		h.Update(func() { r.replaceDone(replaced, changed, errors.Join(errs...)) })
	}()
}

func (r *ReplacePopup) beforeWrite(path string) {
	if r.env.BeforeWrite != nil {
		r.env.BeforeWrite(path)
	}
}

func (r *ReplacePopup) replaceDone(replaced, files int, err error) {
	r.replacing = false
	if err != nil {
		r.env.Host.ShowError("Cannot replace in every file", err)
	}
	if replaced > 0 {
		r.env.Host.Notify(fmt.Sprintf("Replaced %s in %s", plural(replaced, "occurrence"), plural(files, "file")))
	}
	if r.open {
		r.refresh()
	}
}
