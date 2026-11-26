package workspace

import (
	"log"

	"github.com/ZerGo0/editor/internal/design"
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/badge"
	"github.com/ZerGo0/editor/internal/design/chrome"
	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/pane"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/ZerGo0/editor/internal/workspace/notices"
	"github.com/egoist/mygo/ui"
)

const (
	notificationsTitle  = "Notifications"
	noNotifications     = "No new notifications"
	clearNotifications  = "Clear All Notifications"
	hideNotifications   = "Hide Notifications"
	closeNotification   = "Clear Notification"
	showNotificationsID = "notifications.showList"
)

func (w *Workspace) notify(n notices.Notice) string {
	id := w.notes.Add(n)
	if w.win == nil {
		log.Printf("%s: %s", n.Title, n.Detail)
		return id
	}
	w.win.Update(func() {})
	return id
}

func (w *Workspace) setNotice(text string) {
	w.notify(notices.Notice{Tone: token.ToneInfo, Title: text})
}

func (w *Workspace) showError(title string, err error) {
	w.notify(notices.Notice{Tone: token.ToneError, Title: title, Detail: err.Error(), Sticky: true})
}

func (w *Workspace) showNotices(c *ui.Context) {
	show, closed := w.notes.Take()
	for _, id := range closed {
		c.CloseToast(id)
	}
	if w.noteCenter {
		return
	}
	for _, n := range show {
		feedback.Toast(c, feedback.ToastOptions{ID: n.ID, Tone: n.Tone, Title: n.Title, Message: n.Detail, Action: n.Action,
			OnAction: n.OnAction, Sticky: n.Sticky, Busy: n.Busy})
	}
}

func (w *Workspace) ToggleNotifications() {
	w.noteCenter = !w.noteCenter
	if w.noteCenter {
		w.notes.MarkRead()
	}
}

func (w *Workspace) ClearNotifications() {
	w.notes.Clear()
}

func (w *Workspace) notificationsStatus(c *ui.Context) {
	text := ""
	if n := w.notes.Unread(); n > 0 && !w.noteCenter {
		text = badge.CountText(n)
	}
	anchor := chrome.StatusItem(c, text, chrome.StatusOptions{Icon: icons.Bell, ID: "notifications", Command: showNotificationsID,
		Tooltip: notificationsTitle})
	ui.Popover(c, anchor, &w.noteCenter, func() { w.notificationCenter(c) })
}

func (w *Workspace) notificationCenter(c *ui.Context) {
	p := token.Of(c)
	list := w.notes.List()
	ui.Column(c).Width(token.NotificationsWidth.Px()).Role(ui.RoleDialog).Label(notificationsTitle).Children(func() {
		pane.ViewTitle(c, notificationsTitle, pane.TitleOptions{ShowActions: true, Actions: func(c *ui.Context) {
			if action.IconButton(c, icons.ClearAll, clearNotifications, action.IconOptions{Disabled: len(list) == 0}).Clicked() {
				w.ClearNotifications()
			}
			if action.IconButton(c, icons.ChevronDown, hideNotifications, action.IconOptions{}).Clicked() {
				w.noteCenter = false
			}
		}})
		if len(list) == 0 {
			design.Text(c, noNotifications, token.TextLabel).Padding(token.PadInputY.Px(), token.PadView.Px(), token.PadPanel.Px(), token.PadView.Px())
			return
		}
		ui.Scroll(c).FillWidth().MaxHeight(token.NotificationsMax.Px()).Children(func() {
			for _, n := range list {
				w.noticeRow(c, p, n)
			}
		})
	})
}

func (w *Workspace) noticeRow(c *ui.Context, p *token.Palette, n notices.Notice) {
	ui.Row(c.Key(n.ID)).FillWidth().Padding(token.PadNoticeY.Px(), token.PadInputY.Px(), token.PadNoticeY.Px(), token.PadView.Px()).
		Gap(token.GapControl.Px()).AlignItems(ui.Start).BorderWidth(token.BorderWidth.Px(), 0, 0, 0).BorderColor(p.Border.Section).
		Children(func() {
			ui.Icon(c, feedback.ToneIcon(n.Tone)).Size(token.IconSize.Px(), token.IconSize.Px()).TextColor(p.Tone(n.Tone)).Shrink(0).AccessHidden(true)
			ui.Column(c).Grow(1).MinWidth(0).Gap(token.GapTight.Px()).Children(func() {
				design.Text(c, n.Title, token.TextBody).Wrap()
				if n.Detail != "" {
					design.Text(c, n.Detail, token.TextLabel).Wrap().Selectable()
				}
				if n.Busy {
					feedback.ProgressBar(c, n.Title)
				}
				if n.Action != "" && action.Button(c, n.Action, action.ButtonOptions{Variant: action.Secondary, Small: true}).Clicked() {
					w.notes.Close(n.ID)
					n.OnAction()
				}
			})
			if action.IconButton(c, icons.Close, closeNotification, action.IconOptions{ID: "close-" + n.ID}).Clicked() {
				w.notes.Close(n.ID)
			}
		})
}
