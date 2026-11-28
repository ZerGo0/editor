package gitui

import (
	"strings"

	"github.com/ZerGo0/editor/internal/git"
	"github.com/egoist/mygo/ui/editor"
)

func (g *Git) SetCommitTemplate(fn func() string) {
	g.template = fn
}

func (g *Git) OnHeadChange(fn func()) {
	g.headHook = fn
}

func (g *Git) OnStatus(fn func([]git.FileStatus)) {
	g.statusHook = fn
}

func (g *Git) OnToggleWrap(fn func()) {
	g.toggleWrap = fn
}

func (g *Git) ShowCommit(oid string) {
	g.showCommit(oid)
}

func (g *Git) HasChangeAt(path string, buf *editor.Buffer, line int) bool {
	base := g.Base(path)
	if base == nil || buf == nil {
		return false
	}
	d := &editor.DiffState{Old: base, New: buf}
	return hunkAt(d.Hunks(), line) >= 0
}

func (g *Git) applyTemplate() {
	if g.template == nil || g.commit.amend {
		return
	}
	t := g.template()
	if t == "" || t == g.commit.templateUsed {
		return
	}
	if strings.TrimSpace(g.commit.message) == "" {
		g.commit.message = t
	}
	g.commit.templateUsed = t
}
