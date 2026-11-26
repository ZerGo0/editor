package gitui

import (
	"context"
	"fmt"
	"strings"

	"github.com/ZerGo0/editor/internal/git"
	"github.com/egoist/mygo/ui"
)

type stashState struct {
	open      bool
	message   string
	untracked bool
	keepIndex bool
}

type stashListState struct {
	open     bool
	loading  bool
	err      error
	gen      int
	items    []git.Stash
	list     ui.ListState
	selected int
}

func (g *Git) openStash() {
	if !g.ready() {
		return
	}
	g.stash = stashState{open: true}
}

func (g *Git) stashDialog(c *ui.Context) {
	s := &g.stash
	modal(c, &s.open, "Stash Changes", dialogWidth, func() {
		muted(c, "Stash saves all uncommitted changes in Git and reverts them from the work tree.")
		input := ui.TextInput(c.Key("stash-message"), &s.message).AutoFocus().FillWidth().Placeholder("Message").Label("Stash Message")
		ui.Checkbox(c, &s.untracked, "Include untracked files")
		ui.Checkbox(c, &s.keepIndex, "Keep index")
		run := func() {
			s.open = false
			msg, untracked, keep := strings.TrimSpace(s.message), s.untracked, s.keepIndex
			if !g.h.SaveAllFiles() {
				return
			}
			saved := false
			g.doSimple("Stash", func(ctx context.Context, r repo) error {
				var err error
				saved, err = r.StashPush(ctx, msg, untracked, keep)
				return err
			}, func() {
				if !saved {
					g.h.Notify("There are no local changes to stash.")
					return
				}
				g.h.Notify("Stashed the local changes")
				g.reloadAllBuffers()
			})
		}
		if input.Submitted() {
			run()
		}
		dialogButtons(c, func() { s.open = false }, "Create Stash", g.busy != "", run)
	})
}

func (g *Git) openStashes() {
	if !g.ready() {
		return
	}
	g.stashes = stashListState{open: true, selected: -1}
	g.loadStashes()
}

func (g *Git) loadStashes() {
	s := &g.stashes
	s.gen++
	gen, r := s.gen, g.repo
	s.loading, s.err = true, nil
	go func() {
		items, err := r.Stashes(g.ctx)
		g.post(func() {
			if gen != s.gen {
				return
			}
			s.loading, s.err, s.items = false, err, items
			if s.selected >= len(items) || (s.selected < 0 && len(items) > 0) {
				s.selected = min(max(s.selected, 0), len(items)-1)
			}
		})
	}()
}

func (g *Git) stashesDialog(c *ui.Context) {
	s := &g.stashes
	if s.open && !g.ready() {
		s.open = false
	}
	modal(c, &s.open, "Stashes", wideDialog, func() {
		switch {
		case s.err != nil:
			errorState(c, "Cannot load the stashes", s.err, g.loadStashes)
		case s.loading && s.items == nil:
			loadingState(c, "Loading stashes")
		case len(s.items) == 0:
			emptyState(c, "No stashes", "Stash your changes to see them here.")
		default:
			items := s.items
			s.list.Key = func(i int) any { return items[i].OID }
			s.list.Label = func(i int) string { return items[i].Message }
			s.list.Selected = &s.selected
			list := ui.List(c.Key("stashes"), &s.list, len(items)).FillWidth().Height(listHeight).Label("Stashes")
			list.Rows(func(row ui.ListRow) {
				st := items[row.Index]
				rc := row.Context
				ui.Row(rc).Height(rowHeight).FillWidth().Padding(0, rowPadding).Gap(6).AlignItems(ui.Center).Children(func() {
					muted(rc, st.Ref).SingleLine().Shrink(0)
					ui.Text(rc, st.Message).FontSize(bodyFont).SingleLine().Grow(1).Shrink(1).MinWidth(0)
					muted(rc, formatTime(st.Time, rc.Now())).SingleLine().Shrink(0)
				})
			})
			if list.Submitted() {
				g.showStash(s.selectedStash())
			}
		}
		st, ok := s.selectedStash(), s.selected >= 0 && s.selected < len(s.items)
		busy := g.busy != ""
		ui.Row(c).FillWidth().Gap(6).Children(func() {
			if ui.Button(c, "Show Diff").Disabled(!ok).Clicked() {
				g.showStash(st)
			}
			if ui.Button(c, "Drop").Disabled(!ok || busy).Clicked() {
				g.dropStash(st)
			}
			if ui.Button(c, "Clear").Disabled(len(s.items) == 0 || busy).Clicked() {
				g.clearStashes()
			}
			ui.Box(c).Grow(1)
			if ui.Button(c, "Close").Clicked() {
				s.open = false
			}
			if ui.Button(c, "Apply").Disabled(!ok || busy).Clicked() {
				g.applyStash(st, false)
			}
			if ui.PrimaryButton(c, "Pop").Disabled(!ok || busy).Clicked() {
				g.applyStash(st, true)
			}
		})
	})
}

func (s *stashListState) selectedStash() git.Stash {
	if s.selected < 0 || s.selected >= len(s.items) {
		return git.Stash{Index: -1}
	}
	return s.items[s.selected]
}

func (g *Git) showStash(st git.Stash) {
	if st.Index < 0 {
		return
	}
	g.openPatchTab("git-stash:"+st.OID, "Stash: "+firstLineOf(st.Message), func(ctx context.Context, r repo) ([]patchItem, error) {
		fds, err := r.StashShow(ctx, st.Index)
		if err != nil {
			return nil, err
		}
		return fileDiffItems(fds), nil
	})
}

func (g *Git) applyStash(st git.Stash, pop bool) {
	title := "Apply Stash"
	if pop {
		title = "Pop Stash"
	}
	g.stashes.open = false
	g.do(op{
		title:  title,
		banner: true,
		run: func(ctx context.Context, r repo, _ func(string)) (git.OpResult, error) {
			return r.StashApply(ctx, st.Index, pop)
		},
		done: func(res git.OpResult) {
			if !res.Stopped {
				g.h.Notify(fmt.Sprintf("Applied %s", st.Ref))
			}
			g.reloadAllBuffers()
		},
	})
}

func (g *Git) dropStash(st git.Stash) {
	g.confirm(question{
		message:     "Drop " + st.Ref + "?",
		detail:      st.Message,
		action:      "Drop",
		destructive: true,
	}, func() {
		g.doSimple("Drop Stash", func(ctx context.Context, r repo) error { return r.StashDrop(ctx, st.Index) }, g.loadStashes)
	})
}

func (g *Git) clearStashes() {
	n := len(g.stashes.items)
	g.confirm(question{
		message:     fmt.Sprintf("Drop all %d stashes?", n),
		detail:      "The stashed changes are lost.",
		action:      "Clear",
		destructive: true,
	}, func() {
		g.doSimple("Clear Stashes", func(ctx context.Context, r repo) error {
			for range n {
				if err := r.StashDrop(ctx, 0); err != nil {
					return err
				}
			}
			return nil
		}, g.loadStashes)
	})
}
