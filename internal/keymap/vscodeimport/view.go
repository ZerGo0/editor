package vscodeimport

import (
	"github.com/ZerGo0/editor/internal/design"
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

const (
	ViewTitle       = "VS Code Keybindings Import"
	openKeybindings = "Open Keyboard Shortcuts (JSON)"
	noEntriesNotice = "The VS Code keybindings.json has no entries."
)

type Summary struct {
	Report          Report
	OpenKeybindings func()
}

func (s *Summary) View(c *ui.Context) {
	r := s.Report
	design.Page(c, ViewTitle, func() {
		design.Text(c, r.Headline(), token.TextStrong)
		if r.Total == 0 {
			design.Text(c, noEntriesNotice, token.TextParagraph)
		}
		for _, line := range r.Details() {
			design.Text(c, line, token.TextParagraph).Wrap().Selectable()
		}
		if s.OpenKeybindings != nil && action.Button(c, openKeybindings, action.ButtonOptions{}).Clicked() {
			s.OpenKeybindings()
		}
		for _, section := range r.Sections() {
			design.Setting(c, section.Title, section.Note, func() {
				design.Stack(c, token.GapTight, func() {
					for _, row := range section.Rows {
						design.Text(c, row, token.TextLabel).Wrap().Selectable()
					}
				})
			})
		}
	})
}
