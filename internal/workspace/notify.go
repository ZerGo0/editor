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
	"github.com/ZerGo0/editor/internal/errtext"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/ZerGo0/editor/internal/plural"
	"github.com/ZerGo0/editor/internal/workspace/notices"
	"github.com/egoist/mygo/ui"
)

const (
	notificationsTitle  = "Notifications"
	noNotifications     = "No new notifications"
	clearNotifications  = "Clear All Notifications"
	hideNotifications   = "Hide Notifications"
	closeNotification   = "Clear Notification"
	doNotDisturbLabel   = "Toggle Do Not Disturb Mode"
	doNotDisturbTooltip = "Do Not Disturb: show only error notifications as pop-ups"
	showNotificationsID = "notifications.showList"
	cancelActionLabel   = "Cancel"
)

type toastState struct {
	shown        map[string]bool
	hide         bool
	focus        bool
	doNotDisturb bool
}

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

func (w *Workspace) NotifyAction(title, detail, label string, onAction func()) {
	w.notify(notices.Notice{Tone: token.ToneInfo, Title: title, Detail: detail, Actions: []notices.Action{{Label: label, Run: onAction}}, Sticky: true})
}

func (w *Workspace) showError(title string, err error, actions ...notices.Action) {
	log.Printf("%s: %v", title, err)
	w.notify(notices.Notice{Tone: token.ToneError, Title: title, Detail: errtext.Text(err), Raw: err.Error(), Actions: actions, Sticky: true})
}

func (w *Workspace) Warn(title, detail string, actions ...notices.Action) {
	w.notify(notices.Notice{Tone: token.ToneWarning, Title: title, Detail: detail, Actions: actions, Sticky: true})
}

func (w *Workspace) busyNotice(title string, cancel func()) string {
	return w.notify(notices.Notice{Tone: token.ToneInfo, Title: title, Busy: true, Actions: []notices.Action{{Label: cancelActionLabel, Run: cancel}}})
}

func (w *Workspace) progress(title string, cancel func()) func() {
	id := w.busyNotice(title, cancel)
	return func() { w.notes.Close(id) }
}

func (w *Workspace) showNotices(c *ui.Context) {
	t := &w.toasts
	show, closed := w.notes.Take()
	for _, id := range closed {
		c.CloseToast(id)
	}
	if (w.noteCenter || t.hide) && len(t.shown) > 0 {
		c.CloseToast("")
	}
	t.hide = false
	if w.noteCenter {
		return
	}
	if t.shown == nil {
		t.shown = map[string]bool{}
	}
	for _, n := range show {
		if t.doNotDisturb && n.Tone != token.ToneError {
			continue
		}
		o := feedback.ToastOptions{ID: n.ID, Tone: n.Tone, Title: n.Title, Message: n.Detail, Sticky: n.Sticky, Busy: n.Busy,
			OnClose: func() { delete(t.shown, n.ID) }}
		if len(n.Actions) > 0 {
			o.Action, o.OnAction = n.Actions[0].Label, n.Actions[0].Run
		}
		if len(n.Actions) > 1 {
			o.Secondary, o.OnSecondary = n.Actions[1].Label, n.Actions[1].Run
		}
		t.shown[feedback.Toast(c, o)] = true
	}
}

func (w *Workspace) toastHost(c *ui.Context) {
	feedback.ToastHost(c, feedback.ToastHostOptions{Focus: &w.toasts.focus})
}

func (w *Workspace) toastsVisible() bool {
	return len(w.toasts.shown) > 0 && !w.noteCenter
}

func (w *Workspace) HideToasts() {
	w.toasts.hide = true
}

func (w *Workspace) FocusToasts() {
	w.toasts.focus = w.toastsVisible()
}

func (w *Workspace) ToggleDoNotDisturb() {
	w.toasts.doNotDisturb = !w.toasts.doNotDisturb
	if w.toasts.doNotDisturb {
		w.toasts.hide = true
	}
}

func (w *Workspace) ToggleNotifications() {
	w.noteCenter = !w.noteCenter
	if w.noteCenter {
		w.notes.MarkRead()
	}
}

func (w *Workspace) HideNotifications() {
	w.noteCenter = false
}

func (w *Workspace) ClearNotifications() {
	w.notes.Clear()
}

func (w *Workspace) notificationsStatus(c *ui.Context) {
	text, label := "", notificationsTitle
	if n := w.notes.Unread(); n > 0 && !w.noteCenter {
		text, label = badge.CountText(n), plural.Count(n, "New Notification", "New Notifications")
	}
	icon := icons.Bell
	if w.toasts.doNotDisturb {
		icon = icons.BellOff
	}
	anchor := chrome.StatusItem(c, text, chrome.StatusOptions{Icon: icon, ID: "notifications", Command: showNotificationsID,
		Label: label, Tooltip: notificationsTitle})
	var first ui.Element
	ui.Popover(c, anchor, &w.noteCenter, func() { first = w.notificationCenter(c) })
	if w.noteCenter {
		first.AutoFocus()
		if !c.WindowActive() {
			w.noteCenter = false
		}
	}
}

func (w *Workspace) notificationCenter(c *ui.Context) (first ui.Element) {
	p := token.Of(c)
	list := w.notes.List()
	ui.Column(c).Width(token.NotificationsWidth.Px()).Role(ui.RoleDialog).Label(notificationsTitle).Children(func() {
		pane.ViewTitle(c, notificationsTitle, pane.TitleOptions{ShowActions: true, Actions: func(c *ui.Context) {
			clearAll := action.IconButton(c, icons.ClearAll, clearNotifications, action.IconOptions{Disabled: len(list) == 0})
			action.ToggleIconButton(c, icons.BellOff, doNotDisturbLabel, &w.toasts.doNotDisturb, action.IconOptions{Tooltip: doNotDisturbTooltip})
			hide := action.IconButton(c, icons.ChevronDown, hideNotifications, action.IconOptions{})
			first = hide
			if len(list) > 0 {
				first = clearAll
			}
			if clearAll.Clicked() {
				w.ClearNotifications()
			}
			if hide.Clicked() {
				w.HideNotifications()
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
	return first
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
				if n.Raw != "" && n.Raw != n.Detail {
					design.Text(c, n.Raw, token.TextCaption).Wrap().Selectable()
				}
				if n.Busy {
					feedback.ProgressBar(c, n.Title)
				}
				if len(n.Actions) == 0 {
					return
				}
				design.Inline(c, token.GapControl, func() {
					for _, a := range n.Actions {
						if action.Button(c, a.Label, action.ButtonOptions{Variant: action.Secondary, Small: true}).Clicked() {
							w.notes.Close(n.ID)
							a.Run()
						}
					}
				})
			})
			if action.IconButton(c, icons.Close, closeNotification, action.IconOptions{ID: "close-" + n.ID}).Clicked() {
				w.notes.Close(n.ID)
			}
		})
}
