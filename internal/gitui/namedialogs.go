package gitui

import (
	"context"
	"strings"

	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/field"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

const (
	tagMessageMinLines = 2
	tagMessageMaxLines = 6
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
	modal(c, &s.open, "New Branch", token.DialogWidth, func() {
		name := strings.TrimSpace(s.name)
		var problem string
		if name != "" {
			if err := g.validBranchName(name); err != nil {
				problem = err.Error()
			}
		}
		input := field.TextInput(c, "Branch Name", &s.name, field.InputOptions{Placeholder: "Branch name", Error: problem, AutoFocus: true, ID: "new-branch"})
		if s.start != "" {
			secondary(c, "From "+displayRev(s.start))
		}
		action.Checkbox(c, "Checkout branch", &s.checkout, action.ChoiceOptions{})
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
	modal(c, &s.open, "New Tag", token.DialogWidth, func() {
		name, rev := strings.TrimSpace(s.name), strings.TrimSpace(s.rev)
		var problem string
		if name != "" {
			if err := checkName(name); err != nil {
				problem = "Enter a tag name without spaces."
			}
		}
		field.TextInput(c, "Tag Name", &s.name, field.InputOptions{Placeholder: "Tag name", Error: problem, AutoFocus: true, ID: "tag-name"})
		field.TextInput(c, "Tag Commit", &s.rev, field.InputOptions{Placeholder: "Commit, branch or expression", ID: "tag-rev"})
		field.TextArea(c, "Tag Message", &s.message, field.AreaOptions{Placeholder: "Message (an empty message makes a lightweight tag)",
			MinLines: tagMessageMinLines, MaxLines: tagMessageMaxLines, ID: "tag-message"})
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
