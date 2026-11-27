package viewer

import (
	"path/filepath"
	"strings"

	"github.com/ZerGo0/editor/internal/design"
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/detail"
	"github.com/ZerGo0/editor/internal/design/feedback"
	dpane "github.com/ZerGo0/editor/internal/design/pane"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

const (
	infoSeparator = "  \u00b7  "
	toolbarLabel  = "Viewer actions"
)

func (s *State) View(c *ui.Context) ui.Element {
	s.svc = c.Services()
	p := token.Of(c)
	col := ui.Column(c).Fill().Background(p.Layer.Editor)
	col.Children(func() {
		s.header(c)
		s.pane.body(c)
		s.footer(c)
	})
	return col
}

func (s *State) header(c *ui.Context) {
	p := token.Of(c)
	dpane.Toolbar(c, toolbarLabel, func(c *ui.Context) {
		s.pane.controls(c)
		design.Spacer(c)
		if iconButton(c, "reveal", icons.Reveal, "Reveal in Finder", false) {
			s.reveal()
		}
		if action.Button(c, "Open With Default App", action.ButtonOptions{Variant: action.Ghost, Small: true, ID: "open-default"}).Clicked() {
			s.openDefault()
		}
		if action.Button(c, "Copy Path", action.ButtonOptions{Variant: action.Ghost, Small: true, ID: "copy-path"}).Clicked() {
			s.copyPath(c)
		}
	}).Padding(0, token.PadRow.Px()).FillWidth().Background(p.Layer.Panel).BorderWidth(0, 0, token.BorderWidth.Px(), 0).BorderColor(p.Border.Default)
}

func (s *State) footer(c *ui.Context) {
	text := s.pane.info()
	if text == "" {
		return
	}
	p := token.Of(c)
	ui.Row(c).Height(token.StatusBarHeight.Px()).Padding(0, token.PadBarText.Px()).Shrink(0).FillWidth().AlignItems(ui.Center).Background(p.Layer.Panel).
		BorderWidth(token.BorderWidth.Px(), 0, 0, 0).BorderColor(p.Border.Default).Children(func() {
		design.Text(c, text, token.TextCaption).SingleLine()
	})
}

func (s *State) name() string { return filepath.Base(s.path) }

func joinInfo(parts ...string) string {
	kept := parts[:0:0]
	for _, p := range parts {
		if p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, infoSeparator)
}

func message(c *ui.Context, title, description, actionLabel string, onAction func()) {
	detail.Centered(c, func() {
		feedback.Empty(c, feedback.EmptyOptions{Title: title, Message: description, Action: actionLabel, OnAction: onAction})
	})
}

var iconKeys = map[string]string{"Zoom In": "⌘=", "Zoom Out": "⌘-"}

func iconButton(c *ui.Context, key string, svg *ui.SVG, label string, disabled bool) bool {
	return action.IconButton(c, svg, label, action.IconOptions{ID: key, Disabled: disabled, Keys: iconKeys[label]}).Clicked()
}
