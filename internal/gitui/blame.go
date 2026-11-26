package gitui

import (
	"fmt"
	"strings"
	"time"

	"github.com/ZerGo0/editor/internal/git"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	blameEllipsis    = "…"
	blameGap         = "    "
	notCommittedText = "Not committed yet"
)

type blameOptions struct {
	hideAuthor  bool
	hideDate    bool
	showHash    bool
	hideSummary bool
}

type blameState struct {
	path       string
	rel        string
	st         *editor.State
	lines      []git.BlameLine
	loading    bool
	err        error
	gen        int
	prevMenu   func(m *ui.Menu)
	prevGutter func(line int)
}

func (g *Git) Annotated(path string) bool {
	_, ok := g.blames[path]
	return ok
}

func (g *Git) Annotate(path string, st *editor.State) {
	if b, ok := g.blames[path]; ok {
		g.closeBlame(b)
		return
	}
	if !g.ready() || st == nil || st.Buffer == nil {
		return
	}
	rel, ok := g.paths.rel(path)
	if !ok {
		g.h.Notify("This file is not in the repository.")
		return
	}
	if g.isUntracked(rel) {
		g.h.Notify("This file is not tracked by Git yet.")
		return
	}
	b := &blameState{path: path, rel: rel, st: st, prevMenu: st.ContextMenu, prevGutter: st.OnGutterClick}
	g.blames[path] = b
	st.ContextMenu = func(m *ui.Menu) {
		if b.prevMenu != nil {
			b.prevMenu(m)
			m.Separator()
		}
		g.blameMenu(m, b)
	}
	st.OnGutterClick = func(line int) {
		if bl, ok := b.at(line); ok && !bl.IsUncommitted() {
			g.showCommit(bl.OID)
			return
		}
		if b.prevGutter != nil {
			b.prevGutter(line)
		}
	}
	g.runBlame(b)
}

func (g *Git) closeBlame(b *blameState) {
	b.gen++
	b.st.SetInlays(nil)
	b.st.ContextMenu, b.st.OnGutterClick = b.prevMenu, b.prevGutter
	delete(g.blames, b.path)
}

func (g *Git) reblameAll() {
	for _, b := range g.blames {
		g.runBlame(b)
	}
}

func (g *Git) runBlame(b *blameState) {
	if !g.ready() {
		return
	}
	b.gen++
	gen, r, rel := b.gen, g.repo, b.rel
	contents := []byte(b.st.Buffer.String())
	b.loading, b.err = true, nil
	go func() {
		lines, err := r.Blame(g.ctx, rel, "", contents)
		g.post(func() {
			if gen != b.gen {
				return
			}
			b.loading, b.err = false, err
			if err != nil {
				g.h.ShowError("Cannot annotate "+rel, err)
				g.closeBlame(b)
				return
			}
			b.lines = lines
			g.paintBlame(b)
		})
	}()
}

func (b *blameState) at(line int) (git.BlameLine, bool) {
	for _, bl := range b.lines {
		if bl.Line-1 == line {
			return bl, true
		}
	}
	return git.BlameLine{}, false
}

func (g *Git) paintBlame(b *blameState) {
	buf := b.st.Buffer
	now := time.Now()
	inlays := make([]editor.Inlay, 0, len(b.lines))
	for _, bl := range b.lines {
		i := bl.Line - 1
		if i < 0 || i >= buf.LineCount() {
			continue
		}
		off := buf.Offset(editor.Pos{Line: i, Col: len(buf.Line(i))})
		inlays = append(inlays, editor.Inlay{Offset: off, Text: blameGap + blameLabel(bl, g.blameOpts, now), Trailing: true, Ellipsis: blameEllipsis})
	}
	b.st.SetInlays(inlays)
}

func blameLabel(bl git.BlameLine, o blameOptions, now time.Time) string {
	if bl.IsUncommitted() {
		return notCommittedText
	}
	var parts []string
	if !o.hideAuthor {
		parts = append(parts, bl.Author)
	}
	if !o.hideDate {
		parts = append(parts, formatTime(bl.Time, now))
	}
	if o.showHash {
		parts = append(parts, shortHash(bl.OID))
	}
	label := strings.Join(parts, ", ")
	if !o.hideSummary && bl.Summary != "" {
		if label != "" {
			label += " · "
		}
		label += bl.Summary
	}
	return label
}

func (g *Git) blameMenu(m *ui.Menu, b *blameState) {
	line := b.st.CursorPos().Line
	bl, ok := b.at(line)
	committed := ok && !bl.IsUncommitted()
	if m.Item("Show Commit in Log").Disabled(!committed).Chosen() {
		g.showCommit(bl.OID)
	}
	if m.Item("Show Commit Changes").Disabled(!committed).Chosen() {
		g.openCommitOID(bl.OID)
	}
	if m.Item("Copy Revision Number").Disabled(!committed).Chosen() {
		g.clipboard = bl.OID
	}
	m.Submenu("Annotation Options", func(m *ui.Menu) {
		o := &g.blameOpts
		toggle := func(label string, on bool, flip func()) {
			if m.Item(label).Checked(on).Chosen() {
				flip()
				for _, other := range g.blames {
					g.paintBlame(other)
				}
			}
		}
		toggle("Show Author", !o.hideAuthor, func() { o.hideAuthor = !o.hideAuthor })
		toggle("Show Date", !o.hideDate, func() { o.hideDate = !o.hideDate })
		toggle("Show Commit Hash", o.showHash, func() { o.showHash = !o.showHash })
		toggle("Show Commit Message", !o.hideSummary, func() { o.hideSummary = !o.hideSummary })
	})
	if m.Item("Refresh Annotations").Chosen() {
		g.runBlame(b)
	}
	if m.Item("Close Annotations").Chosen() {
		g.closeBlame(b)
	}
}

func (g *Git) openCommitOID(oid string) {
	if !g.ready() {
		return
	}
	r := g.repo
	go func() {
		commits, err := r.Log(g.ctx, git.LogOptions{Revs: []string{oid}, Max: 1})
		if err == nil && len(commits) == 0 {
			err = fmt.Errorf("commit %s not found", shortHash(oid))
		}
		g.post(func() {
			if err != nil {
				g.h.ShowError("Cannot open the commit", err)
				return
			}
			g.openCommitTab(commits[0])
		})
	}()
}
