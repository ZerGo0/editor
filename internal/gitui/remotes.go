package gitui

import (
	"context"
	"strings"

	"github.com/ZerGo0/editor/internal/design"
	"github.com/ZerGo0/editor/internal/design/dialog"
	"github.com/ZerGo0/editor/internal/design/field"
	"github.com/ZerGo0/editor/internal/design/list"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

type remotesState struct {
	open     bool
	list     ui.ListState
	selected int
	editing  bool
	original string
	name     string
	url      string
}

func (g *Git) openRemotes() {
	if !g.ready() {
		return
	}
	g.remotes = remotesState{open: true, selected: -1}
	g.refreshNow()
}

func (g *Git) remotesDialog(c *ui.Context) {
	s := &g.remotes
	if s.open && !g.ready() {
		s.open = false
	}
	modal(c, &s.open, "Manage Remotes", token.DialogWidth, func() {
		if s.editing {
			g.remoteForm(c)
			return
		}
		remotes := g.snap.remotes
		if len(remotes) == 0 {
			emptyState(c, "No remotes", "Add a remote to fetch and push.")
		} else {
			s.list.Key = func(i int) any { return remotes[i].Name }
			s.list.Label = func(i int) string { return remotes[i].Name }
			s.list.Selected = &s.selected
			boxedList(c, "Remotes", &s.list, len(remotes), func(i int) list.Item { return g.remoteItem(remotes[i]) })
		}
		dialog.Actions(c, func() {
			if button(c, "Add Remote...", false) {
				s.editing, s.original, s.name, s.url = true, "", "", ""
			}
			design.Spacer(c)
			if button(c, "Close", false) {
				s.open = false
			}
		})
	})
}

func (g *Git) remoteItem(rm git.Remote) list.Item {
	s := &g.remotes
	return list.Item{
		Label:       rm.Name,
		Strong:      true,
		Icon:        icons.Github,
		Description: rm.FetchURL,
		Tooltip:     rm.FetchURL,
		Actions: func(c *ui.Context) {
			if iconButton(c, "Edit Remote "+rm.Name, icons.Edit, false) {
				s.editing, s.original, s.name, s.url = true, rm.Name, rm.Name, rm.FetchURL
			}
			if iconButton(c, "Remove Remote "+rm.Name, icons.Close, g.busy != "") {
				g.removeRemote(rm.Name)
			}
		},
		Menu: func(m *ui.Menu) {
			if m.Item("Edit Remote...").Chosen() {
				s.editing, s.original, s.name, s.url = true, rm.Name, rm.Name, rm.FetchURL
			}
			if m.Item("Remove Remote").Danger().Disabled(g.busy != "").Chosen() {
				g.removeRemote(rm.Name)
			}
		},
	}
}

func (g *Git) remoteForm(c *ui.Context) {
	s := &g.remotes
	field.TextInput(c, "Remote Name", &s.name, field.InputOptions{Placeholder: "Name", ID: "remote-form-name"})
	field.TextInput(c, "Remote URL", &s.url, field.InputOptions{Placeholder: "URL", AutoFocus: true, ID: "remote-form-url"})
	name, url := strings.TrimSpace(s.name), strings.TrimSpace(s.url)
	disabled := checkName(name) != nil || checkURL(url) != nil
	action := "Add"
	if s.original != "" {
		action = "Save"
	}
	dialogButtons(c, func() { s.editing = false }, action, disabled, func() {
		s.editing = false
		original := s.original
		g.doSimple("Save Remote", func(ctx context.Context, r repo) error {
			if original == "" {
				return r.AddRemote(ctx, name, url)
			}
			if original != name {
				if err := r.RenameRemote(ctx, original, name); err != nil {
					return err
				}
			}
			return r.SetRemoteURL(ctx, name, url)
		}, nil)
	})
}

func (g *Git) removeRemote(name string) {
	g.confirm(question{
		message:     "Remove remote " + quoted(name) + "?",
		detail:      "Its remote-tracking branches are removed too.",
		action:      "Remove",
		destructive: true,
	}, func() {
		g.doSimple("Remove Remote", func(ctx context.Context, r repo) error { return r.RemoveRemote(ctx, name) }, nil)
	})
}
