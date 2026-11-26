package feedback

import (
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

const (
	dismissLabel  = "Dismiss"
	stickyTimeout = -1
)

type NoticeOptions struct {
	Tone      token.Tone
	Text      string
	Action    string
	OnAction  func()
	OnDismiss func()
}

func Notice(c *ui.Context, o NoticeOptions) ui.Element {
	p := token.Of(c)
	bg, border := noticeColors(p, o.Tone)
	role := ui.RoleStatus
	if o.Tone == token.ToneError {
		role = ui.RoleAlert
	}
	return ui.Row(c).FillWidth().Padding(token.PadNoticeY.Px(), token.PadNoticeX.Px()).Gap(token.GapControl.Px()).AlignItems(ui.Center).
		Radius(token.RadiusControl.Px()).Background(bg).Border(token.BorderWidth.Px(), border).Role(role).Label(o.Text).Children(func() {
		ui.Icon(c, ToneIcon(o.Tone)).Size(token.IconSize.Px(), token.IconSize.Px()).TextColor(p.Tone(o.Tone)).Shrink(0).AccessHidden(true)
		token.TextLabel.Apply(ui.Text(c, o.Text)).TextColor(p.Text.Primary).Wrap().Grow(1).MinWidth(0)
		if o.Action != "" && o.OnAction != nil {
			if action.Button(c, o.Action, action.ButtonOptions{Small: true}).Clicked() {
				o.OnAction()
			}
		}
		if o.OnDismiss != nil {
			if action.IconButton(c, icons.Close, dismissLabel, action.IconOptions{}).Clicked() {
				o.OnDismiss()
			}
		}
	})
}

func ToneIcon(t token.Tone) *ui.SVG {
	switch t {
	case token.ToneError:
		return icons.Error
	case token.ToneWarning:
		return icons.Warning
	case token.ToneSuccess:
		return icons.Check
	}
	return icons.Info
}

func noticeColors(p *token.Palette, t token.Tone) (bg, border ui.Color) {
	switch t {
	case token.ToneError:
		return p.Severity.ErrorBackground, p.Severity.ErrorBorder
	case token.ToneWarning:
		return p.Severity.WarningBackground, p.Severity.WarningBorder
	case token.ToneInfo:
		return p.Severity.InfoBackground, p.Severity.InfoBorder
	}
	return p.Layer.Widget, p.Border.Widget
}

type ToastOptions struct {
	Tone     token.Tone
	Title    string
	Message  string
	Action   string
	OnAction func()
	ID       string
	Sticky   bool
	Busy     bool
	OnClose  func()
}

func Toast(c *ui.Context, o ToastOptions) string {
	t := ui.Toast{ID: o.ID, Title: o.Title, Description: o.Message, Type: o.Tone.String(), Action: o.Action, OnAction: o.OnAction, OnClose: o.OnClose}
	if o.Sticky || o.Busy {
		t.Timeout = stickyTimeout
	}
	if o.Busy {
		t.Data = toastBusy{}
	}
	return c.AddToast(t)
}
