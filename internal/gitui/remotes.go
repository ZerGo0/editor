package gitui

import (
	"context"
	"strings"

	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

const remoteNameWidth = 120

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
	modal(c, &s.open, "Manage Remotes", dialogWidth, func() {
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
			ui.List(c.Key("remotes"), &s.list, len(remotes)).FillWidth().Height(listHeight).Label("Remotes").Rows(func(row ui.ListRow) {
				g.remoteRow(row, remotes[row.Index])
			})
		}
		ui.Row(c).FillWidth().Gap(6).Children(func() {
			if ui.Button(c, "Add Remote...").Clicked() {
				s.editing, s.original, s.name, s.url = true, "", "", ""
			}
			ui.Box(c).Grow(1)
			if ui.Button(c, "Close").Clicked() {
				s.open = false
			}
		})
	})
}

func (g *Git) remoteRow(row ui.ListRow, rm git.Remote) {
	c := row.Context
	s := &g.remotes
	ui.Row(c).Height(rowHeight).FillWidth().Padding(0, rowPadding).Gap(6).AlignItems(ui.Center).Children(func() {
		ui.Text(c, rm.Name).FontSize(bodyFont).Bold().SingleLine().Width(remoteNameWidth).Shrink(0)
		muted(c, rm.FetchURL).SingleLine().Grow(1).Shrink(1).MinWidth(0)
		if iconButton(c, "Edit Remote "+rm.Name, icons.Edit, false).Clicked() {
			s.editing, s.original, s.name, s.url = true, rm.Name, rm.Name, rm.FetchURL
		}
		if iconButton(c, "Remove Remote "+rm.Name, icons.Close, g.busy != "").Clicked() {
			g.removeRemote(rm.Name)
		}
	})
}

func (g *Git) remoteForm(c *ui.Context) {
	s := &g.remotes
	ui.TextInput(c.Key("remote-form-name"), &s.name).FillWidth().Placeholder("Name").Label("Remote Name")
	ui.TextInput(c.Key("remote-form-url"), &s.url).AutoFocus().FillWidth().Placeholder("URL").Label("Remote URL")
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
