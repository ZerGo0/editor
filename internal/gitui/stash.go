package gitui

import (
	"context"
	"fmt"
	"strings"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/design"
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/dialog"
	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/field"
	"github.com/ZerGo0/editor/internal/design/list"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/ZerGo0/editor/internal/plural"
	"github.com/egoist/mygo/ui"
)

type stashState struct {
	open      bool
	message   string
	untracked bool
	keepIndex bool
}

type stashListState struct {
	listState[git.Stash]
}

func (g *Git) openStash() {
	if !g.ready() {
		return
	}
	g.stash = stashState{open: true}
}

func (g *Git) stashDialog(c *ui.Context) {
	s := &g.stash
	modal(c, &s.open, "Stash Changes", token.DialogWidth, func() {
		paragraph(c, "Stash saves all uncommitted changes in Git and reverts them from the work tree.")
		input := field.TextInput(c, "Stash Message", &s.message, field.InputOptions{Placeholder: "Message", AutoFocus: true, ID: "stash-message"})
		action.Checkbox(c, "Include untracked files", &s.untracked, action.ChoiceOptions{})
		action.Checkbox(c, "Keep index", &s.keepIndex, action.ChoiceOptions{})
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
	g.stashes = stashListState{listState: openList[git.Stash]()}
	g.loadStashes()
}

func (g *Git) loadStashes() {
	s := &g.stashes
	s.gen++
	gen, r := s.begin(), g.repo
	applog.Go(func() {
		items, err := r.Stashes(g.ctx)
		g.post(func() {
			if s.apply(gen, items, err) {
				s.clampSelection()
			}
		})
	})
}

func (g *Git) stashesDialog(c *ui.Context) {
	s := &g.stashes
	if s.open && !g.ready() {
		s.open = false
	}
	modal(c, &s.open, "Stashes", token.DialogWidthWide, func() {
		switch {
		case s.err != nil:
			errorState(c, "Cannot load the stashes", s.err, g.loadStashes)
		case s.loading && s.items == nil:
			boxedSkeleton(c, "Loading stashes", func(i int) list.Item {
				return stashItem(git.Stash{Ref: placeholderWord(i), Message: placeholderSubject(i), Time: placeholderTime(c.Now(), i)}, nil)
			})
		case len(s.items) == 0:
			feedback.Empty(c, feedback.EmptyOptions{Title: "No stashes", Message: "Stash your changes to see them here.", Action: "Stash Changes...", OnAction: func() {
				s.open = false
				g.openStash()
			}})
		default:
			items := s.items
			s.list.Key = func(i int) any { return items[i].OID }
			s.list.Label = func(i int) string { return items[i].Message }
			s.list.Selected = &s.selected
			l := boxedList(c, "Stashes", &s.list, len(items), func(i int) list.Item { return stashItem(items[i], g.showStash) })
			if l.Submitted() {
				g.showStash(s.selectedStash())
			}
		}
		st, ok := s.selectedStash(), s.selected >= 0 && s.selected < len(s.items)
		busy := g.busy != ""
		dialog.Actions(c, func() {
			if button(c, "Show Diff", !ok) {
				g.showStash(st)
			}
			if button(c, "Drop", !ok || busy) {
				g.dropStash(st)
			}
			if button(c, "Drop All", len(s.items) == 0 || busy) {
				g.clearStashes()
			}
			design.Spacer(c)
			if button(c, "Close", false) {
				s.open = false
			}
			if button(c, "Apply", !ok || busy) {
				g.applyStash(st, false)
			}
			if primaryButton(c, "Pop", !ok || busy) {
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
	g.openPatchTab("git-stash:"+st.OID, "Stash: "+git.FirstLine(st.Message), func(ctx context.Context, r repo) ([]patchItem, error) {
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
		message:     "Drop all " + plural.Count(n, "stash", "stashes") + "?",
		detail:      "The stashed changes are lost.",
		action:      "Drop All",
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

func stashItem(st git.Stash, show func(git.Stash)) list.Item {
	return list.Item{Label: st.Message, Icon: icons.Stash, Description: st.Ref,
		Trailing:      func(c *ui.Context) { secondary(c, formatTime(st.Time, c.Now())).SingleLine() },
		OnDoubleClick: func() { show(st) }}
}
