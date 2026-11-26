package vscodeimport

import (
	"strings"

	"github.com/ZerGo0/editor/internal/keymap"
	"github.com/egoist/mygo/ui"
)

const (
	fontBody    = 13
	fontSmall   = 12
	pagePadding = 20
	sectionGap  = 12
	rowGap      = 4
	keyWidth    = 180
	contentMax  = 900
)

type Summary struct {
	Loading         bool
	Report          Report
	Err             error
	OpenKeybindings func()
	Retry           func()
}

func (s *Summary) View(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Background(t.Background).Children(func() {
		switch {
		case s.Loading:
			ui.Column(c).Grow(1).FillWidth().Center().Gap(sectionGap).Children(func() {
				ui.Spinner(c).Label("Importing")
				ui.Text(c, "Importing VS Code keybindings...").FontSize(fontBody).TextColor(t.TextMuted)
			})
		case s.Err != nil:
			ui.Column(c).Grow(1).FillWidth().Center().Gap(sectionGap).Padding(pagePadding).Children(func() {
				ui.Text(c, "Cannot import VS Code keybindings").FontSize(fontBody).Bold()
				ui.Text(c, s.Err.Error()).FontSize(fontSmall).TextColor(t.Danger)
				if s.Retry != nil {
					ui.Button(c, "Try Again").OnClick(s.Retry)
				}
			})
		default:
			ui.Scroll(c).Grow(1).FillWidth().Children(func() {
				ui.Column(c).Gap(sectionGap).Padding(pagePadding).MaxWidth(contentMax).FillWidth().Children(func() { s.body(c) })
			})
		}
	})
}

func (s *Summary) body(c *ui.Context) {
	t := c.Theme()
	r := s.Report
	ui.Text(c, r.Headline()).FontSize(fontBody).Bold()
	if r.Total == 0 {
		ui.Text(c, "The VS Code keybindings.json has no entries.").FontSize(fontSmall).TextColor(t.TextMuted)
	}
	for _, line := range r.Details() {
		ui.Text(c, line).FontSize(fontSmall).TextColor(t.TextMuted)
	}
	if s.OpenKeybindings != nil {
		ui.Row(c).Children(func() {
			ui.Button(c, "Open Keyboard Shortcuts (JSON)").OnClick(s.OpenKeybindings)
		})
	}
	if len(r.Plan.Inactive) > 0 {
		section(c, "Imported but inactive", "These need a context that this editor does not have, so they do not run.")
		for _, e := range r.Plan.Inactive {
			entryRow(c, e, "when "+e.When)
		}
	}
	if len(r.Plan.Skipped) > 0 {
		section(c, "Not imported", "")
		for _, sk := range r.Plan.Skipped {
			entryRow(c, sk.Entry, sk.Reason)
		}
	}
}

func section(c *ui.Context, title, note string) {
	t := c.Theme()
	ui.Column(c).Gap(rowGap).FillWidth().Children(func() {
		ui.Text(c, title).FontSize(fontBody).Bold()
		if note != "" {
			ui.Text(c, note).FontSize(fontSmall).TextColor(t.TextMuted)
		}
	})
}

func entryRow(c *ui.Context, e keymap.Entry, reason string) {
	t := c.Theme()
	label := strings.TrimSpace(e.Key + " " + e.Command)
	ui.Row(c).Gap(sectionGap).FillWidth().Label(label + ": " + reason).Children(func() {
		ui.Text(c, e.Key).FontSize(fontSmall).Width(keyWidth).Shrink(0)
		ui.Text(c, e.Command).FontSize(fontSmall).Grow(1).MinWidth(0)
		ui.Text(c, reason).FontSize(fontSmall).TextColor(t.TextMuted).Grow(1).MinWidth(0)
	})
}
