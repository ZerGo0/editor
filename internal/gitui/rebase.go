package gitui

import (
	"context"
	"strings"

	"github.com/ZerGo0/editor/internal/git"
	"github.com/egoist/mygo/ui"
)

const actionWidth = 96

type rebaseState struct {
	open     bool
	onto     string
	loading  bool
	err      error
	gen      int
	items    []git.TodoItem
	actions  []string
	messages map[string]string
	list     ui.ListState
	selected int
}

func (g *Git) startRebase() {
	if !g.ready() {
		return
	}
	if st := g.snap.status; st != nil && st.Upstream != "" {
		g.openRebase(st.Upstream)
		return
	}
	g.openBranchPicker("Rebase onto", g.openRebase)
}

func (g *Git) openRebase(onto string) {
	if !g.ready() {
		return
	}
	g.rebase = rebaseState{open: true, onto: onto, selected: -1, messages: map[string]string{}}
	g.loadTodo()
}

func (g *Git) loadTodo() {
	s := &g.rebase
	s.gen++
	gen, r, onto := s.gen, g.repo, s.onto
	s.loading, s.err = true, nil
	go func() {
		items, err := r.RebaseTodo(g.ctx, onto)
		g.post(func() {
			if gen != s.gen {
				return
			}
			s.loading, s.err, s.items = false, err, items
			s.actions = make([]string, len(items))
			for i, it := range items {
				s.actions[i] = string(it.Action)
			}
			if len(items) > 0 {
				s.selected = 0
			}
		})
	}()
}

func (g *Git) rebaseDialog(c *ui.Context) {
	s := &g.rebase
	if s.open && !g.ready() {
		s.open = false
	}
	modal(c, &s.open, "Rebase Commits onto "+displayRev(s.onto), wideDialog, func() {
		switch {
		case s.err != nil:
			errorState(c, "Cannot list the commits to rebase", s.err, g.loadTodo)
			return
		case s.loading:
			loadingState(c, "Loading commits")
			return
		case len(s.items) == 0:
			emptyState(c, "No commits to rebase", "The current branch has no commits after "+displayRev(s.onto)+".")
			dialogButtons(c, func() { s.open = false }, "Start Rebasing", true, func() {})
			return
		}
		g.todoList(c)
		g.rewordEditor(c)
		_, problem := buildTodo(s.items, s.actions)
		if problem != nil {
			ui.Text(c, problem.Error()).FontSize(smallFont).TextColor(c.Theme().Danger)
		}
		dialogButtons(c, func() { s.open = false }, "Start Rebasing", problem != nil || g.busy != "", g.runRebase)
	})
}

func (g *Git) todoList(c *ui.Context) {
	s := &g.rebase
	items := s.items
	ui.Row(c).FillWidth().Gap(6).Children(func() {
		muted(c, "Commits are applied from top to bottom.").Grow(1)
		if textButton(c, "Move Up", s.selected <= 0).Clicked() {
			s.selected = moveTodo(s.items, s.actions, s.selected, -1)
		}
		if textButton(c, "Move Down", s.selected < 0 || s.selected >= len(items)-1).Clicked() {
			s.selected = moveTodo(s.items, s.actions, s.selected, 1)
		}
	})
	s.list.Key = func(i int) any { return items[i].OID }
	s.list.Label = func(i int) string { return items[i].Subject }
	s.list.Selected = &s.selected
	list := ui.List(c.Key("todo"), &s.list, len(items)).FillWidth().Height(listHeight).Label("Rebase Commits")
	list.Rows(func(row ui.ListRow) {
		i := row.Index
		it := items[i]
		rc := row.Context
		ui.Row(rc).Height(rowHeight+4).FillWidth().Padding(0, rowPadding).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Select(rc.Key("action"), &s.actions[i], todoActions).Width(actionWidth).Label("Action for " + shortHash(it.OID)).Shrink(0)
			subject := ui.Text(rc, s.subject(i)).FontSize(bodyFont).SingleLine().Grow(1).Shrink(1).MinWidth(0)
			if s.actions[i] == string(git.ActionDrop) {
				subject.Strikethrough().TextColor(rc.Theme().TextMuted)
			}
			muted(rc, shortHash(it.OID)).SingleLine().Shrink(0)
		})
	})
	if list.Shortcut(ui.Alt, ui.KeyUp) {
		s.selected = moveTodo(s.items, s.actions, s.selected, -1)
	}
	if list.Shortcut(ui.Alt, ui.KeyDown) {
		s.selected = moveTodo(s.items, s.actions, s.selected, 1)
	}
}

func (s *rebaseState) subject(i int) string {
	if msg, ok := s.messages[s.items[i].OID]; ok && s.actions[i] == string(git.ActionReword) {
		return firstLineOf(msg)
	}
	return s.items[i].Subject
}

func (g *Git) rewordEditor(c *ui.Context) {
	s := &g.rebase
	if s.selected < 0 || s.selected >= len(s.items) || s.actions[s.selected] != string(git.ActionReword) {
		return
	}
	it := s.items[s.selected]
	msg, ok := s.messages[it.OID]
	if !ok {
		msg = it.Subject
	}
	muted(c, "New message for "+shortHash(it.OID))
	if ui.TextArea(c.Key("reword-"+it.OID), &msg).Lines(2, 6).FillWidth().Label("New Commit Message").Changed() || !ok {
		s.messages[it.OID] = msg
	}
}

func (g *Git) runRebase() {
	s := &g.rebase
	items, err := buildTodo(s.items, s.actions)
	if err != nil {
		return
	}
	messages := map[string]string{}
	for _, it := range items {
		if msg := strings.TrimSpace(s.messages[it.OID]); it.Action == git.ActionReword && msg != "" {
			messages[it.OID] = msg
		}
	}
	onto := s.onto
	s.open = false
	g.do(op{
		title:  "Rebase",
		banner: true,
		run: func(ctx context.Context, r repo, _ func(string)) (git.OpResult, error) {
			return r.RebaseInteractive(ctx, onto, items, messages)
		},
		done: func(res git.OpResult) {
			if !res.Stopped {
				g.h.Notify("Rebased " + g.headLabel())
			}
			g.headChanged()
		},
	})
}
