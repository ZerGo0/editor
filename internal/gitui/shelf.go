package gitui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/design"
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/dialog"
	"github.com/ZerGo0/editor/internal/design/list"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	shelfCapBytes   = 200 << 20
	bytesPerMB      = 1 << 20
	shelfNameFormat = "Shelved on Jan 2 15:04"
)

var errShelfName = errors.New("Enter a name for the shelved changes.")

type shelfState struct {
	open     bool
	loading  bool
	err      error
	gen      int
	items    []git.ShelfEntry
	list     ui.ListState
	selected int
	remove   bool
}

type errShelfFull struct {
	used, need int64
}

func (e errShelfFull) Error() string {
	return fmt.Sprintf("The shelf is limited to %d MB. It uses %d MB and these changes need about %d MB. Delete old shelved changes first.",
		shelfCapBytes/bytesPerMB, e.used/bytesPerMB, e.need/bytesPerMB)
}

func checkShelfRoom(used, need int64) error {
	if used+need > shelfCapBytes {
		return errShelfFull{used: used, need: need}
	}
	return nil
}

func estimateShelf(root pathMap, paths []string) int64 {
	var total int64
	for _, rel := range paths {
		if info, err := os.Stat(root.abs(rel)); err == nil && info.Mode().IsRegular() {
			total += info.Size()
		}
	}
	return total
}

func (g *Git) changedTrackedPaths() []string {
	if g.snap.status == nil {
		return nil
	}
	seen := map[string]bool{}
	var paths []string
	add := func(files []git.FileStatus) {
		for _, f := range files {
			if !seen[f.Path] {
				seen[f.Path] = true
				paths = append(paths, f.Path)
			}
		}
	}
	add(g.snap.status.Staged())
	add(g.snap.status.Unstaged())
	return paths
}

func (g *Git) askShelve(paths []string) {
	if !g.ready() {
		return
	}
	if len(paths) == 0 {
		g.h.Notify("There are no tracked changes to shelve.")
		return
	}
	g.h.Ask("Shelve Changes", "Shelve", time.Now().Format(shelfNameFormat), func(name string) error {
		name = strings.TrimSpace(name)
		if name == "" {
			return errShelfName
		}
		if !g.h.SaveAllFiles() {
			return nil
		}
		need := estimateShelf(g.paths, paths)
		g.doSimple("Shelve", func(ctx context.Context, r repo) error {
			used, err := r.ShelfUsage()
			if err != nil {
				return err
			}
			if err := checkShelfRoom(used, need); err != nil {
				return err
			}
			_, err = r.Shelve(ctx, name, paths)
			return err
		}, func() {
			g.h.Notify("Shelved " + name)
			g.reloadBuffers(paths)
			if g.shelf.open {
				g.loadShelf()
			}
		})
		return nil
	})
}

func (g *Git) openShelf() {
	if !g.ready() {
		return
	}
	g.shelf = shelfState{open: true, selected: -1, remove: true}
	g.loadShelf()
}

func (g *Git) loadShelf() {
	s := &g.shelf
	s.gen++
	gen, r := s.gen, g.repo
	s.loading, s.err = true, nil
	applog.Go(func() {
		items, err := r.ShelfList()
		g.post(func() {
			if gen != s.gen {
				return
			}
			s.loading, s.err, s.items = false, err, items
			if s.selected >= len(items) || (s.selected < 0 && len(items) > 0) {
				s.selected = min(max(s.selected, 0), len(items)-1)
			}
		})
	})
}

func (g *Git) shelfDialog(c *ui.Context) {
	s := &g.shelf
	if s.open && !g.ready() {
		s.open = false
	}
	modal(c, &s.open, "Shelf", token.DialogWidthWide, func() {
		paragraph(c, "The shelf keeps patches of chosen files in the editor. They are not stored in Git.")
		switch {
		case s.err != nil:
			errorState(c, "Cannot load the shelf", s.err, g.loadShelf)
		case s.loading && s.items == nil:
			loadingState(c, "Loading the shelf")
		case len(s.items) == 0:
			emptyState(c, "The shelf is empty", "Shelve changes from the Commit window to see them here.")
		default:
			items := s.items
			s.list.Key = func(i int) any { return items[i].ID }
			s.list.Label = func(i int) string { return items[i].Name }
			s.list.Selected = &s.selected
			l := boxedList(c, "Shelved Changes", &s.list, len(items), func(i int) list.Item {
				e := items[i]
				return list.Item{Label: e.Name, Icon: icons.Stash, Trailing: func(c *ui.Context) { secondary(c, formatTime(e.Created, c.Now())).SingleLine() },
					OnDoubleClick: func() { g.showShelf(e) }}
			})
			if l.Submitted() {
				g.showShelf(s.selectedEntry())
			}
		}
		e, ok := s.selectedEntry(), s.selected >= 0 && s.selected < len(s.items)
		busy := g.busy != ""
		action.Checkbox(c, "Remove from the shelf after unshelve", &s.remove, action.ChoiceOptions{})
		dialog.Actions(c, func() {
			if button(c, "Show Diff", !ok) {
				g.showShelf(e)
			}
			if button(c, "Delete", !ok || busy) {
				g.deleteShelf(e)
			}
			design.Spacer(c)
			if button(c, "Close", false) {
				s.open = false
			}
			if primaryButton(c, "Unshelve", !ok || busy) {
				g.unshelve(e, s.remove)
			}
		})
	})
}

func (s *shelfState) selectedEntry() git.ShelfEntry {
	if s.selected < 0 || s.selected >= len(s.items) {
		return git.ShelfEntry{}
	}
	return s.items[s.selected]
}

func (g *Git) showShelf(e git.ShelfEntry) {
	if e.ID == "" {
		return
	}
	g.openPatchTab("git-shelf:"+e.ID, "Shelf: "+e.Name, func(ctx context.Context, r repo) ([]patchItem, error) {
		data, err := r.ShelfPatch(e.ID)
		if err != nil {
			return nil, err
		}
		fps, err := editor.ParsePatch(string(data))
		if err != nil {
			return nil, fmt.Errorf("shelf %s: %w", e.Name, err)
		}
		return filePatchItems(fps), nil
	})
}

func filePatchItems(fps []editor.FilePatch) []patchItem {
	items := make([]patchItem, 0, len(fps))
	for i := range fps {
		fp := fps[i]
		it := patchItem{title: fp.NewName, code: 'M', patch: &fp}
		switch {
		case fp.OldName == "":
			it.code = 'A'
		case fp.NewName == "":
			it.code, it.title = 'D', fp.OldName
		case fp.OldName != fp.NewName:
			it.code, it.detail = 'R', fp.OldName
		}
		if fp.Binary {
			it.patch, it.note = nil, errBinary.Error()
		}
		items = append(items, it)
	}
	return items
}

func (g *Git) unshelve(e git.ShelfEntry, remove bool) {
	g.shelf.open = false
	g.do(op{
		title:  "Unshelve",
		banner: true,
		run: func(ctx context.Context, r repo, _ func(string)) (git.OpResult, error) {
			return r.Unshelve(ctx, e.ID, remove)
		},
		done: func(res git.OpResult) {
			if !res.Stopped {
				g.h.Notify("Unshelved " + e.Name)
			}
			g.reloadAllBuffers()
		},
	})
}

func (g *Git) deleteShelf(e git.ShelfEntry) {
	g.confirm(question{
		message:     "Delete the shelved changes " + quoted(e.Name) + "?",
		detail:      "The patch is deleted from the shelf.",
		action:      "Delete",
		destructive: true,
	}, func() {
		g.doSimple("Delete Shelf", func(_ context.Context, r repo) error { return r.ShelfDelete(e.ID) }, g.loadShelf)
	})
}
