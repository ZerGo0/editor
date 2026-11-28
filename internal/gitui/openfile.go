package gitui

import (
	"os"
	"slices"

	"github.com/ZerGo0/editor/internal/multidiff"
)

const noFileToOpen = "Open a diff or select a changed file first."

type activeViewer interface {
	ActiveViewID() string
}

type sourceControlFocus interface {
	SourceControlFocused() bool
}

type sourceLine struct {
	rel  string
	line int
}

func (g *Git) openWorkingFile() {
	targets := g.diffTargets()
	if len(targets) == 0 {
		for _, rel := range g.selectedFiles() {
			targets = append(targets, sourceLine{rel: rel, line: -1})
		}
	}
	g.openSources(targets)
}

func (g *Git) openSources(targets []sourceLine) {
	opened := false
	for _, t := range targets {
		abs := g.paths.abs(t.rel)
		if _, err := os.Stat(abs); err != nil {
			continue
		}
		col := -1
		if t.line >= 0 {
			col = 0
		}
		g.h.OpenAt(abs, t.line, col)
		opened = true
	}
	if !opened {
		g.h.Notify(noFileToOpen)
	}
}

func (g *Git) jumpToSource(paths ...string) {
	targets := make([]sourceLine, 0, len(paths))
	for _, rel := range paths {
		targets = append(targets, sourceLine{rel: rel, line: -1})
	}
	g.openSources(targets)
}

func (g *Git) diffTargets() []sourceLine {
	id := g.activeView()
	if t, ok := g.tabs[id]; ok {
		line := -1
		if t.state != nil {
			line = multidiff.NewSideLine(t.state)
		}
		return []sourceLine{{rel: t.src.path, line: line}}
	}
	if t, ok := g.patchTabs[id]; ok {
		if rel, line, ok := t.view.Caret(); ok {
			return []sourceLine{{rel: rel, line: line}}
		}
	}
	return nil
}

func (g *Git) selectedFiles() []string {
	if f, ok := g.h.(sourceControlFocus); !ok || !f.SourceControlFocused() {
		return nil
	}
	s := &g.changes
	if s.cursor < 0 || s.cursor >= len(s.entries) || s.entries[s.cursor].kind != entryFile {
		return nil
	}
	return rowFilePaths(g.targets(s.entries[s.cursor]))
}

func rowFilePaths(rows []changeRow) []string {
	var paths []string
	for _, r := range rows {
		if !slices.Contains(paths, r.file.Path) {
			paths = append(paths, r.file.Path)
		}
	}
	return paths
}
