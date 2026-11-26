package gitui

import (
	"context"
	"strings"

	"github.com/egoist/mygo/ui"
)

type newBranchState struct {
	open     bool
	name     string
	start    string
	checkout bool
}

type tagState struct {
	open    bool
	name    string
	rev     string
	message string
}

func (g *Git) newBranch(start string) {
	if !g.ready() {
		return
	}
	g.newBranchDlg = newBranchState{open: true, start: start, checkout: true}
}

func (g *Git) newBranchDialog(c *ui.Context) {
	s := &g.newBranchDlg
	modal(c, &s.open, "New Branch", dialogWidth, func() {
		name := strings.TrimSpace(s.name)
		var problem string
		if name != "" {
			if err := g.validBranchName(name); err != nil {
				problem = err.Error()
			}
		}
		input := ui.TextInput(c.Key("new-branch"), &s.name).AutoFocus().FillWidth().Placeholder("Branch name").Label("Branch Name")
		if problem != "" {
			ui.Text(c, problem).FontSize(smallFont).TextColor(c.Theme().Danger)
		}
		if s.start != "" {
			muted(c, "From "+displayRev(s.start))
		}
		ui.Checkbox(c, &s.checkout, "Checkout branch")
		create := func() {
			if name == "" || problem != "" {
				return
			}
			s.open = false
			start, checkout := s.start, s.checkout
			g.doSimple("New Branch", func(ctx context.Context, r repo) error {
				return r.CreateBranch(ctx, name, start, checkout)
			}, func() {
				g.h.Notify("Created branch " + name)
				if checkout {
					g.headChanged()
				}
			})
		}
		if input.Submitted() {
			create()
		}
		dialogButtons(c, func() { s.open = false }, "Create", name == "" || problem != "", create)
	})
}

func displayRev(rev string) string {
	if isHashLike(rev) {
		return shortHash(rev)
	}
	return rev
}

func (g *Git) newTag(rev string) {
	if !g.ready() {
		return
	}
	if rev == "" {
		rev = headRev
	}
	g.tagDlg = tagState{open: true, rev: rev}
}

func (g *Git) tagDialog(c *ui.Context) {
	s := &g.tagDlg
	modal(c, &s.open, "New Tag", dialogWidth, func() {
		name, rev := strings.TrimSpace(s.name), strings.TrimSpace(s.rev)
		var problem string
		if name != "" {
			if err := checkName(name); err != nil {
				problem = "Enter a tag name without spaces."
			}
		}
		ui.TextInput(c.Key("tag-name"), &s.name).AutoFocus().FillWidth().Placeholder("Tag name").Label("Tag Name")
		if problem != "" {
			ui.Text(c, problem).FontSize(smallFont).TextColor(c.Theme().Danger)
		}
		ui.TextInput(c.Key("tag-rev"), &s.rev).FillWidth().Placeholder("Commit, branch or expression").Label("Tag Commit")
		ui.TextArea(c.Key("tag-message"), &s.message).Lines(2, 6).FillWidth().
			Placeholder("Message (an empty message makes a lightweight tag)").Label("Tag Message")
		disabled := name == "" || rev == "" || problem != ""
		dialogButtons(c, func() { s.open = false }, "Create Tag", disabled, func() {
			s.open = false
			msg := s.message
			g.doSimple("New Tag", func(ctx context.Context, r repo) error { return r.CreateTag(ctx, name, rev, msg) },
				func() {
					g.h.Notify("Created tag " + name)
					if g.log.started {
						g.log.reload()
					}
				})
		})
	})
}
