package gitui

import (
	"os"
	"slices"
)

const noFileToOpen = "Open a diff or select a changed file first."

type activeViewer interface {
	ActiveViewID() string
}

type sourceControlFocus interface {
	SourceControlFocused() bool
}

func (g *Git) openWorkingFile() {
	paths := g.diffFiles()
	if len(paths) == 0 {
		paths = g.selectedFiles()
	}
	opened := false
	for _, rel := range paths {
		abs := g.paths.abs(rel)
		if _, err := os.Stat(abs); err != nil {
			continue
		}
		g.h.OpenAt(abs, -1, -1)
		opened = true
	}
	if !opened {
		g.h.Notify(noFileToOpen)
	}
}

func (g *Git) diffFiles() []string {
	v, ok := g.h.(activeViewer)
	if !ok {
		return nil
	}
	id := v.ActiveViewID()
	if t, ok := g.tabs[id]; ok {
		return []string{t.src.path}
	}
	if t, ok := g.patchTabs[id]; ok {
		if rel, ok := t.view.CurrentPath(); ok {
			return []string{rel}
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
	var paths []string
	for _, r := range g.targets(s.entries[s.cursor]) {
		if !slices.Contains(paths, r.file.Path) {
			paths = append(paths, r.file.Path)
		}
	}
	return paths
}
