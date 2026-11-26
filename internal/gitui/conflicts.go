package gitui

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/ZerGo0/editor/internal/git"
	"github.com/egoist/mygo/ui"
)

const sideColumnWidth = 90

type conflictsState struct {
	open     bool
	loading  bool
	err      error
	gen      int
	items    []git.ConflictFile
	list     ui.ListState
	selected int
	auto     bool
}

func (g *Git) openConflicts() {
	if !g.ready() {
		return
	}
	g.conflicts = conflictsState{open: true, selected: -1}
	g.loadConflicts()
}

func (g *Git) loadConflicts() {
	s := &g.conflicts
	s.gen++
	gen, r := s.gen, g.repo
	s.loading, s.err = true, nil
	go func() {
		items, err := r.Conflicts(g.ctx)
		g.post(func() {
			if gen != s.gen {
				return
			}
			s.loading, s.err, s.items = false, err, items
			if s.auto && err == nil && len(items) == 0 {
				s.open = false
			}
			s.auto = false
			if s.selected >= len(items) || (s.selected < 0 && len(items) > 0) {
				s.selected = min(max(s.selected, 0), len(items)-1)
			}
		})
	}()
}

func (g *Git) openConflictsIfAny() {
	if !g.ready() {
		return
	}
	g.conflicts = conflictsState{open: true, selected: -1, auto: true}
	g.loadConflicts()
}

func sideState(present bool) string {
	if present {
		return "Modified"
	}
	return "Deleted"
}

func (g *Git) rebasing() bool {
	return g.snap.op.Kind == git.StateRebasing || g.snap.op.Kind == git.StateRebaseInteractive
}

func (g *Git) conflictsDialog(c *ui.Context) {
	s := &g.conflicts
	if s.open && !g.ready() {
		s.open = false
	}
	modal(c, &s.open, "Conflicts", wideDialog, func() {
		if g.rebasing() {
			muted(c, "During a rebase, Yours is the branch you rebase onto and Theirs is your commit.")
		}
		switch {
		case s.err != nil:
			errorState(c, "Cannot load the conflicts", s.err, g.loadConflicts)
		case s.loading && s.items == nil:
			loadingState(c, "Loading conflicts")
		case len(s.items) == 0:
			emptyState(c, "No conflicts", "All conflicts are resolved.")
		default:
			g.conflictList(c)
		}
		f, ok := s.selectedFile()
		busy := g.busy != ""
		ui.Row(c).FillWidth().Gap(6).Children(func() {
			if ui.Button(c, "Accept Yours").Disabled(!ok || busy).Clicked() {
				g.acceptConflict(f.Path, true)
			}
			if ui.Button(c, "Accept Theirs").Disabled(!ok || busy).Clicked() {
				g.acceptConflict(f.Path, false)
			}
			if ui.Button(c, "Resolve Simple Conflicts").Disabled(len(s.items) == 0 || busy).Clicked() {
				g.resolveSimple(s.items)
			}
			ui.Box(c).Grow(1)
			if ui.Button(c, "Close").Clicked() {
				s.open = false
			}
			if ui.PrimaryButton(c, "Merge...").Disabled(!ok || !f.Ours || !f.Theirs).Clicked() {
				s.open = false
				g.openMergeTool(f.Path)
			}
		})
	})
}

func (g *Git) conflictList(c *ui.Context) {
	s := &g.conflicts
	items := s.items
	s.list.Key = func(i int) any { return items[i].Path }
	s.list.Label = func(i int) string { return items[i].Path }
	s.list.Selected = &s.selected
	ui.Row(c).FillWidth().Padding(0, rowPadding).Gap(6).Children(func() {
		sectionTitle(c, "Name")
		ui.Box(c).Grow(1)
		sectionTitle(c, "Yours").Width(sideColumnWidth)
		sectionTitle(c, "Theirs").Width(sideColumnWidth)
	})
	list := ui.List(c.Key("conflicts"), &s.list, len(items)).FillWidth().Height(listHeight).Label("Conflicted Files")
	list.Rows(func(row ui.ListRow) {
		f := items[row.Index]
		rc := row.Context
		el := ui.Row(rc).Height(rowHeight).FillWidth().Padding(0, rowPadding).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Text(rc, f.Path).FontSize(bodyFont).SingleLine().Grow(1).Shrink(1).MinWidth(0).TruncateStart()
			muted(rc, sideState(f.Ours)).Width(sideColumnWidth).Shrink(0)
			muted(rc, sideState(f.Theirs)).Width(sideColumnWidth).Shrink(0)
		})
		if el.RowDoubleClicked() && f.Ours && f.Theirs {
			s.open = false
			g.openMergeTool(f.Path)
		}
	})
}

func (s *conflictsState) selectedFile() (git.ConflictFile, bool) {
	if s.selected < 0 || s.selected >= len(s.items) {
		return git.ConflictFile{}, false
	}
	return s.items[s.selected], true
}

func (g *Git) acceptConflict(rel string, ours bool) {
	g.doSimple("Resolve", func(ctx context.Context, r repo) error { return r.AcceptSide(ctx, rel, ours) }, func() {
		g.reloadBuffers([]string{rel})
		g.loadConflicts()
	})
}

func (g *Git) resolveSimple(files []git.ConflictFile) {
	resolved := 0
	g.doSimple("Resolve Simple Conflicts", func(ctx context.Context, r repo) error {
		for _, f := range files {
			if !f.Base || !f.Ours || !f.Theirs {
				continue
			}
			ok, err := resolveClean(ctx, r, f.Path)
			if err != nil {
				return err
			}
			if ok {
				resolved++
			}
		}
		return nil
	}, func() {
		g.h.Notify(fmt.Sprintf("Resolved %d of %d files", resolved, len(files)))
		g.loadConflicts()
		g.reloadAllBuffers()
	})
}

func resolveClean(ctx context.Context, r repo, rel string) (bool, error) {
	base, ours, theirs, err := r.MergeVersions(ctx, rel)
	if err != nil {
		return false, err
	}
	for _, data := range [][]byte{base, ours, theirs} {
		if checkText(data) != nil {
			return false, nil
		}
	}
	m := newMergeModel(string(base), string(ours), string(theirs))
	if !m.clean() {
		return false, nil
	}
	abs := filepath.Join(r.WorkDir(), filepath.FromSlash(rel))
	mode := fs.FileMode(mergeFileMode)
	if info, err := os.Stat(abs); err == nil {
		mode = info.Mode().Perm()
	}
	if err := os.WriteFile(abs, []byte(m.result()), mode); err != nil {
		return false, fmt.Errorf("write %s: %w", rel, err)
	}
	return true, r.MarkResolved(ctx, rel)
}
