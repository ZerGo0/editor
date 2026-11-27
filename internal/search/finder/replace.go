package finder

import (
	"errors"
	"fmt"

	"github.com/ZerGo0/editor/internal/applog"
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

type replacementCache struct {
	matcher  *textsearch.Matcher
	with     string
	preserve bool
	repl     textsearch.Replacement
}

func (d *Dialog) replaceVisible() bool {
	return d.tab == TabText && d.st.ShowReplace
}

func (d *Dialog) canReplace() bool {
	return d.replaceVisible() && d.text.matcher != nil && d.text.err == "" && d.text.total > 0 && !d.replacing && !d.text.searching
}

func (d *Dialog) replacement() textsearch.Replacement {
	c := &d.replCache
	if c.matcher != d.text.matcher || c.with != d.st.With || c.preserve != d.st.PreserveCase || c.matcher == nil {
		*c = replacementCache{matcher: d.text.matcher, with: d.st.With, preserve: d.st.PreserveCase, repl: d.text.matcher.Replacement(d.st.With, d.st.PreserveCase)}
	}
	return c.repl
}

func (d *Dialog) replaceSelected() {
	if e, ok := d.current(); ok && e.kind == kindMatch {
		d.replaceMatch(e.file, e.match)
	}
}

func (d *Dialog) replaceMatch(f *FileNode, i int) { d.replaceIn(f, f.Matches[i:i+1]) }

func (d *Dialog) replaceFile(f *FileNode) { d.replaceIn(f, f.Matches) }

func (d *Dialog) replaceIn(f *FileNode, matches []textsearch.Match) {
	if d.canReplace() {
		d.moved = true
		d.apply([]replaceTarget{{file: f, accept: acceptSet(matches)}})
	}
}

func (d *Dialog) askReplaceAll() {
	d.confirm = d.canReplace()
}

func (d *Dialog) replaceAll() {
	if !d.canReplace() {
		return
	}
	targets := make([]replaceTarget, len(d.text.files))
	for i, f := range d.text.files {
		targets[i] = replaceTarget{file: f, accept: acceptSet(f.Matches)}
	}
	d.apply(targets)
}

func (d *Dialog) apply(targets []replaceTarget) {
	m, repl := d.text.matcher, d.replacement()
	h := d.env.Host
	d.replacing = true
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
	applog.Go(func() {
		var errs []error
		for _, t := range disk {
			path := t.file.Path
			n, err := textsearch.ReplaceFile(path, m, repl, t.accept, 0, func() { d.beforeWrite(path) })
			if err != nil {
				errs = append(errs, err)
				continue
			}
			if n > 0 {
				replaced += n
				changed++
			}
		}
		h.Update(func() { d.replaceDone(replaced, changed, errors.Join(errs...)) })
	})
}

func (d *Dialog) beforeWrite(path string) {
	if d.env.BeforeWrite != nil {
		d.env.BeforeWrite(path)
	}
}

func (d *Dialog) replaceDone(replaced, files int, err error) {
	d.replacing = false
	if err != nil {
		d.env.Host.ShowError("Cannot replace in every file", err)
	}
	if replaced > 0 {
		d.env.Host.Notify(fmt.Sprintf("Replaced %s in %s", plural(replaced, "occurrence"), plural(files, "file")))
	}
	if d.open {
		d.refresh()
	}
}
