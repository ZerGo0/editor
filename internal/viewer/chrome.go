package viewer

import (
	"path/filepath"
	"strings"

	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

const (
	bodyFontSize    = 13
	smallFontSize   = 12
	headerPadY      = 2
	headerPadX      = 8
	footerPadY      = 4
	footerPadX      = 12
	infoSeparator   = "  \u00b7  "
	iconButtonSize  = 22
	iconSize        = 12
	messageGap      = 8
	messagePad      = 24
	messageMaxWidth = 480
	buttonRadius    = 5
	toolbarLabel    = "Viewer actions"
)

func (s *State) View(c *ui.Context) ui.Element {
	s.svc = c.Services()
	t := c.Theme()
	col := ui.Column(c).Fill().Background(t.Background)
	col.Children(func() {
		s.header(c)
		s.pane.body(c)
		s.footer(c)
	})
	return col
}

func (s *State) header(c *ui.Context) {
	t := c.Theme()
	ui.Toolbar(c, func() {
		s.pane.controls(c)
		ui.Spacer(c)
		if iconButton(c, "reveal", icons.Reveal, "Reveal in Finder", false) {
			s.reveal()
		}
		if ui.Button(c.Key("open-default"), "Open With Default App").FontSize(smallFontSize).Clicked() {
			s.openDefault()
		}
		if ui.Button(c.Key("copy-path"), "Copy Path").FontSize(smallFontSize).Clicked() {
			s.copyPath(c)
		}
	}).Label(toolbarLabel).Padding(headerPadY, headerPadX).Shrink(0).FillWidth().Background(t.Surface).
		BorderWidth(0, 0, 1, 0).BorderColor(t.Border)
}

func (s *State) footer(c *ui.Context) {
	text := s.pane.info()
	if text == "" {
		return
	}
	t := c.Theme()
	ui.Row(c).Padding(footerPadY, footerPadX).Shrink(0).FillWidth().BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Children(func() {
		ui.Text(c, text).FontSize(smallFontSize).TextColor(t.TextMuted).SingleLine()
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

func message(c *ui.Context, title, detail string, actions func()) {
	t := c.Theme()
	ui.Column(c).Grow(1).FillWidth().Center().Gap(messageGap).Padding(messagePad).Children(func() {
		ui.Text(c, title).Bold().FontSize(bodyFontSize).TextAlign(ui.Center)
		if detail != "" {
			ui.Text(c, detail).FontSize(smallFontSize).TextColor(t.TextMuted).TextAlign(ui.Center).MaxWidth(messageMaxWidth)
		}
		if actions != nil {
			ui.Row(c).Gap(messageGap).Children(actions)
		}
	})
}

func loadingView(c *ui.Context, label string) {
	t := c.Theme()
	ui.Column(c).Grow(1).FillWidth().Center().Gap(messageGap).Children(func() {
		ui.Spinner(c).Label(label)
		ui.Text(c, label).FontSize(smallFontSize).TextColor(t.TextMuted)
	})
}

func errorView(c *ui.Context, title string, err error, retry func()) {
	message(c, title, err.Error(), func() {
		if ui.Button(c.Key("retry"), "Retry").Clicked() {
			retry()
		}
	})
}

func iconButton(c *ui.Context, key string, svg *ui.SVG, label string, disabled bool) bool {
	t := c.Theme()
	b := ui.ButtonBase(c.Key(key)).Size(iconButtonSize, iconButtonSize).Radius(buttonRadius).Center().
		TextColor(t.TextMuted).Label(label).Tooltip(label).Disabled(disabled)
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() { ui.Icon(c, svg).Size(iconSize, iconSize) })
	return b.Clicked()
}
