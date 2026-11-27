package gitui

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/design"
	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/list"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/git"
	"github.com/egoist/mygo/ui"
)

type conflictsState struct {
	listState[git.ConflictFile]
	auto bool
}

func (g *Git) openConflicts() { g.showConflicts(false) }

func (g *Git) loadConflicts() {
	s := &g.conflicts
	s.gen++
	gen, r := s.begin(), g.repo
	applog.Go(func() {
		items, err := r.Conflicts(g.ctx)
		g.post(func() {
			if !s.apply(gen, items, err) {
				return
			}
			if s.auto && err == nil && len(items) == 0 {
				s.open = false
			}
			s.auto = false
			s.clampSelection()
		})
	})
}

func (g *Git) openConflictsIfAny() { g.showConflicts(true) }

func (g *Git) showConflicts(auto bool) {
	if !g.ready() {
		return
	}
	g.conflicts = conflictsState{listState: openList[git.ConflictFile](), auto: auto}
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
	modal(c, &s.open, "Conflicts", token.DialogWidthWide, func() {
		if g.rebasing() {
			feedback.Notice(c, feedback.NoticeOptions{Tone: token.ToneInfo, Text: "During a rebase, Yours is the branch you rebase onto and Theirs is your commit."})
		}
		switch {
		case s.err != nil:
			errorState(c, "Cannot load the conflicts", s.err, g.loadConflicts)
		case s.loading && s.items == nil:
			conflictHeader(c)
			boxedSkeleton(c, "Loading conflicts", func(i int) list.Item {
				return conflictItem(git.ConflictFile{Path: placeholderPath(i), Ours: true, Theirs: true}, nil)
			})
		case len(s.items) == 0:
			emptyState(c, "No conflicts", "All conflicts are resolved.")
		default:
			g.conflictList(c)
		}
		f, ok := s.selectedFile()
		busy := g.busy != ""
		ui.Row(c).FillWidth().Gap(token.GapControl.Px()).Wrap().Children(func() {
			if button(c, "Accept Yours", !ok || busy) {
				g.acceptSide(f.Path, true, true)
			}
			if button(c, "Accept Theirs", !ok || busy) {
				g.acceptSide(f.Path, false, true)
			}
			if button(c, "Resolve Simple Conflicts", len(s.items) == 0 || busy) {
				g.resolveSimple(s.items)
			}
			design.Spacer(c)
			if button(c, "Close", false) {
				s.open = false
			}
			if primaryButton(c, "Merge...", !ok || !f.Ours || !f.Theirs) {
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
	conflictHeader(c)
	boxedList(c, "Conflicted Files", &s.list, len(items), func(i int) list.Item {
		f := items[i]
		return conflictItem(f, func() {
			s.open = false
			g.openMergeTool(f.Path)
		})
	})
}

func conflictHeader(c *ui.Context) {
	ui.Row(c).FillWidth().Padding(token.Space0.Px(), token.PadRow.Px()).Gap(token.GapControl.Px()).Children(func() {
		secondary(c, "Name").Grow(1)
		secondary(c, "Yours").Width(token.ColumnMedium.Px())
		secondary(c, "Theirs").Width(token.ColumnMedium.Px())
	})
}

func conflictItem(f git.ConflictFile, merge func()) list.Item {
	it := changedFileItem(f.Path, codeConflict, func(bool, bool) {})
	it.OnClick = nil
	it.OnDoubleClick = func() {
		if f.Ours && f.Theirs {
			merge()
		}
	}
	it.Trailing = func(c *ui.Context) {
		secondary(c, sideState(f.Ours)).Width(token.ColumnMedium.Px()).Shrink(0)
		secondary(c, sideState(f.Theirs)).Width(token.ColumnMedium.Px()).Shrink(0)
	}
	return it
}

func (s *conflictsState) selectedFile() (git.ConflictFile, bool) {
	if s.selected < 0 || s.selected >= len(s.items) {
		return git.ConflictFile{}, false
	}
	return s.items[s.selected], true
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
