package gitui

import (
	"context"
	"strings"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/field"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

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
	applog.Go(func() {
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
	})
}

func (g *Git) rebaseDialog(c *ui.Context) {
	s := &g.rebase
	if s.open && !g.ready() {
		s.open = false
	}
	modal(c, &s.open, "Rebase Commits onto "+displayRev(s.onto), token.DialogWidthWide, func() {
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
			errorText(c, problem.Error())
		}
		dialogButtons(c, func() { s.open = false }, "Start Rebasing", problem != nil || g.busy != "", g.runRebase)
	})
}

func (g *Git) todoList(c *ui.Context) {
	s := &g.rebase
	items := s.items
	ui.Row(c).FillWidth().Gap(token.GapTight.Px()).AlignItems(ui.Center).Children(func() {
		secondary(c, "Commits are applied from top to bottom.").Grow(1)
		if iconButton(c, "Move Up", icons.ChevronUp, s.selected <= 0) {
			s.selected = moveTodo(s.items, s.actions, s.selected, -1)
		}
		if iconButton(c, "Move Down", icons.ChevronDown, s.selected < 0 || s.selected >= len(items)-1) {
			s.selected = moveTodo(s.items, s.actions, s.selected, 1)
		}
	})
	s.list.Key = func(i int) any { return items[i].OID }
	s.list.Label = func(i int) string { return items[i].Subject }
	s.list.Selected = &s.selected
	options := make([]action.Option[string], len(todoActions))
	for i, a := range todoActions {
		options[i] = action.Option[string]{Value: a, Label: a}
	}
	p := token.Of(c)
	var list ui.Element
	ui.Column(c).FillWidth().Height(token.ListHeightMax.Px()).Radius(token.RadiusControl.Px()).Clip().
		Border(token.BorderWidth.Px(), p.Border.Input).Children(func() {
		list = ui.List(c.Key("todo"), &s.list, len(items)).Fill().Label("Rebase Commits").Background(ui.Transparent)
		list.Rows(func(row ui.ListRow) {
			i := row.Index
			it := items[i]
			rc := row.Context
			el := ui.Row(rc).FillWidth().Padding(token.Space2.Px(), token.PadRow.Px()).Gap(token.GapControl.Px()).AlignItems(ui.Center)
			switch {
			case row.Selected() && row.ListFocused():
				el.Background(p.List.ActiveSelection)
			case row.Selected():
				el.Background(p.List.InactiveSelection)
			case row.Hovered():
				el.Background(p.List.Hover)
			}
			el.Children(func() {
				ui.Row(rc).Width(token.ColumnMedium.Px()).Shrink(0).Children(func() {
					action.Select(rc, "Action for "+shortHash(it.OID), &s.actions[i], options, action.SelectOptions{ID: "action"})
				})
				subject := token.TextBody.Apply(ui.Text(rc, s.subject(i))).TextColor(p.Text.Primary).SingleLine().Grow(1).Shrink(1).MinWidth(0)
				if s.actions[i] == string(git.ActionDrop) {
					subject.Strikethrough().TextColor(p.Text.Muted)
				}
				secondary(rc, shortHash(it.OID)).SingleLine().Shrink(0)
			})
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
	secondary(c, "New message for "+shortHash(it.OID))
	in := field.TextArea(c, "New Commit Message", &msg, field.AreaOptions{MinLines: tagMessageMinLines, MaxLines: tagMessageMaxLines, ID: "reword-" + it.OID})
	if in.Changed() || !ok {
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
