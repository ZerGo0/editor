package searchview

import (
	"errors"
	"fmt"

	"github.com/ZerGo0/editor/internal/search/textsearch"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui/editor"
)

type replaceTarget struct {
	file   *fileNode
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

func (v *View) canReplace() bool {
	return v.scope == ScopeText && v.text.matcher != nil && v.text.err == "" && v.text.total > 0 && !v.replacing && !v.text.searching
}

func (v *View) replacement() textsearch.Replacement {
	return v.text.matcher.Replacement(v.st.Replace, v.st.PreserveCase)
}

func (v *View) replaceMatch(f *fileNode, i int) {
	if !v.canReplace() || i >= len(f.matches) {
		return
	}
	v.applyReplace([]replaceTarget{{file: f, accept: acceptSet(f.matches[i : i+1])}}, false)
}

func (v *View) showsReplace(r row) bool {
	return v.scope == ScopeText && v.st.ShowReplace && (r.kind == rowMatch || r.kind == rowFile)
}

func (v *View) replaceRow(r row) {
	if r.kind == rowMatch {
		v.replaceMatch(r.file, r.match)
		return
	}
	v.replaceFile(r.file)
}

func (v *View) replaceFile(f *fileNode) {
	if !v.canReplace() {
		return
	}
	v.applyReplace([]replaceTarget{{file: f, accept: acceptSet(f.matches)}}, false)
}

func (v *View) ReplaceAll() {
	if !v.canReplace() {
		return
	}
	targets := make([]replaceTarget, len(v.text.files))
	for i, f := range v.text.files {
		targets[i] = replaceTarget{file: f, accept: acceptSet(f.matches)}
	}
	message := "Replace " + plural(v.text.total, "occurrence") + " across " + plural(len(v.text.files), "file") + "?"
	v.confirm(message, "Open files change in their editors and stay unsaved. Other files are written to disk.", "Replace All", func(ok bool) {
		if ok {
			v.applyReplace(targets, true)
		}
	})
}

func (v *View) applyReplace(targets []replaceTarget, full bool) {
	m, repl := v.text.matcher, v.replacement()
	v.replacing = true
	var disk []replaceTarget
	replaced, changed := 0, 0
	paths := make([]string, 0, len(targets))
	for _, t := range targets {
		paths = append(paths, t.file.path)
		buf := v.h.Buffer(t.file.path)
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
			path := t.file.path
			n, err := textsearch.ReplaceFile(path, m, repl, t.accept, 0, func() { v.beforeWrite(path) })
			if err != nil {
				errs = append(errs, err)
				continue
			}
			if n > 0 {
				replaced += n
				changed++
			}
		}
		v.post(func() { v.replaceDone(replaced, changed, errors.Join(errs...), paths, full) })
	}()
}

func (v *View) beforeWrite(path string) {
	if v.env.BeforeWrite != nil {
		v.env.BeforeWrite(path)
	}
}

func (v *View) replaceDone(replaced, files int, err error, paths []string, full bool) {
	v.replacing = false
	if err != nil {
		v.h.ShowError("Cannot replace in every file", err)
	}
	if replaced > 0 {
		v.h.Notify(fmt.Sprintf("Replaced %s in %s", plural(replaced, "occurrence"), plural(files, "file")))
	}
	if full {
		v.searchSoon(0)
		return
	}
	v.refreshFiles(paths)
}

func (v *View) defaultConfirm(message, detail, action string, done func(ok bool)) {
	win := v.h.Window()
	go func() {
		res, err := mygo.Dialog.Message(mygo.MessageOptions{
			Parent:       win,
			Type:         mygo.MessageWarning,
			Message:      message,
			Detail:       detail,
			Buttons:      []string{action, "Cancel"},
			CancelButton: 1,
		})
		ok := err == nil && res.Button == 0
		v.post(func() { done(ok) })
	}()
}
