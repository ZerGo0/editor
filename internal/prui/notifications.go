package prui

import (
	"fmt"
	"strconv"
	"time"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/list"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/ghshared"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

type notifState struct {
	items     []Notification
	loaded    bool
	loading   bool
	err       error
	modified  string
	poll      time.Duration
	fetchedAt time.Time
	tickAt    time.Time
	gen       int
	list      ui.ListState
	row       int
}

func (p *PRs) NotificationsTitle() string {
	return notificationsName
}

func (p *PRs) NotificationsBadge() int {
	if p.Closed || p.Phase != ghshared.PhaseReady {
		return 0
	}
	n := 0
	for _, item := range p.notifs.items {
		if item.Unread {
			n++
		}
	}
	return n
}

func (p *PRs) loadNotifications() {
	ns := &p.notifs
	ns.gen++
	gen := ns.gen
	ns.loading = true
	be, repo, modified := p.be, p.repo, ns.modified
	if !ns.loaded {
		modified = ""
	}
	applog.Go(func() {
		page, err := be.Notifications(p.Ctx, repo, modified)
		p.Post(func() {
			if gen != p.notifs.gen {
				return
			}
			ns := &p.notifs
			ns.loading = false
			ns.fetchedAt = p.env.Now()
			if err != nil {
				ns.err = err
				p.AuthFailure(err)
				return
			}
			ns.err, ns.loaded = nil, true
			ns.poll = page.Poll
			if page.Modified != "" {
				ns.modified = page.Modified
			}
			if !page.NotModified {
				ns.items = page.Items
			}
		})
	})
}

func (p *PRs) notificationsDue(c *ui.Context) {
	ns := &p.notifs
	if ns.loading {
		return
	}
	now := p.env.Now()
	poll := ns.poll
	if poll <= 0 {
		poll = minPollInterval
	}
	due := ns.fetchedAt.Add(poll)
	if ns.fetchedAt.IsZero() || !now.Before(due) {
		p.loadNotifications()
		return
	}
	if !ns.tickAt.Equal(due) {
		ns.tickAt = due
		c.After(due.Sub(now))
	}
}

func (p *PRs) markNotification(n Notification, done bool) {
	be := p.be
	id := n.ID
	applog.Go(func() {
		err := be.MarkNotification(p.Ctx, id, done)
		p.Post(func() {
			if err != nil {
				p.Host.ShowError("Cannot update the notification", err)
				return
			}
			for i := range p.notifs.items {
				if p.notifs.items[i].ID != id {
					continue
				}
				if done {
					p.notifs.items = append(p.notifs.items[:i], p.notifs.items[i+1:]...)
				} else {
					p.notifs.items[i].Unread = false
				}
				return
			}
		})
	})
}

func (p *PRs) openNotification(n Notification) {
	p.openPR(n.Number, nil)
	if n.Unread {
		p.markNotification(n, false)
	}
}

func (p *PRs) NotificationsActions(c *ui.Context) {
	if p.Ready() && action.IconButton(c, icons.Refresh, "Refresh notifications", action.IconOptions{ID: "refresh-notifications"}).Clicked() {
		p.notifs.fetchedAt = time.Time{}
		p.notifs.modified = ""
	}
}

func (p *PRs) NotificationsLoading() bool {
	return p.Ready() && p.notifs.loading
}

func (p *PRs) NotificationsView(c *ui.Context) {
	if p.Closed {
		return
	}
	if p.Phase != ghshared.PhaseReady {
		p.stateView(c)
		return
	}
	p.notificationsDue(c)
	ns := &p.notifs
	ui.Column(c).Grow(1).FillWidth().MinHeight(0).Children(func() {
		switch {
		case ns.err != nil && len(ns.items) == 0:
			ui.Column(c).FillWidth().Padding(token.GapInline.Px(), token.PadRow.Px()).Children(func() {
				ghshared.ErrorNotice(c, ns.err, func() { ns.fetchedAt = time.Time{} })
			})
		case !ns.loaded:
			feedback.Loading(c, "Loading notifications...")
		case len(ns.items) == 0:
			feedback.Empty(c, feedback.EmptyOptions{Icon: icons.Check, Title: "You are all caught up", Message: "New pull request activity shows here."})
		default:
			ns.list.Selected = &ns.row
			ns.list.Key = func(i int) any { return ns.items[i].ID }
			ns.list.Label = func(i int) string {
				return fmt.Sprintf("%s, #%d, %s", ns.items[i].Title, ns.items[i].Number, ns.items[i].Reason)
			}
			el := list.View(c, &ns.list, len(ns.items), list.ViewOptions{Label: "Notifications"}, func(i int) list.Item { return p.notificationItem(c, ns.items[i]) })
			if el.Submitted() && ns.row >= 0 && ns.row < len(ns.items) {
				p.openNotification(ns.items[ns.row])
			}
		}
	})
}

func (p *PRs) notificationItem(c *ui.Context, n Notification) list.Item {
	pal := token.Of(c)
	description := "#" + strconv.Itoa(n.Number) + prMetaSeparator + reasonLabel(n.Reason)
	if when := ghshared.RelativeTime(p.env.Now(), n.Updated); when != "" {
		description += prMetaSeparator + when
	}
	return list.Item{
		Label:       n.Title,
		Strong:      n.Unread,
		Description: description,
		Tooltip:     n.Title,
		Leading: func(c *ui.Context) {
			dot := ui.Icon(c, icons.Dot).Size(token.IconSizeSmall.Px(), token.IconSizeSmall.Px()).Shrink(0)
			if n.Unread {
				dot.TextColor(pal.Accent.Base).Label("Unread")
			} else {
				dot.TextColor(ui.Transparent).AccessHidden(true)
			}
		},
		OnClick: func() { p.openNotification(n) },
		Menu: func(m *ui.Menu) {
			if action.Item(m, "Open Pull Request", action.ItemOptions{}).Chosen() {
				p.openNotification(n)
			}
			if action.Item(m, "Open on GitHub", action.ItemOptions{}).Chosen() {
				openExternal(c, pullURL(p.repo, n.Number))
			}
			m.Separator()
			if action.Item(m, "Mark as Read", action.ItemOptions{Disabled: !n.Unread}).Chosen() {
				p.markNotification(n, false)
			}
			if action.Item(m, "Mark as Done", action.ItemOptions{}).Chosen() {
				p.markNotification(n, true)
			}
		},
	}
}

func reasonLabel(reason string) string {
	switch reason {
	case "review_requested":
		return "review requested"
	case "mention":
		return "mentioned"
	case "team_mention":
		return "team mentioned"
	case "author":
		return "your pull request"
	case "assign":
		return "assigned"
	case "ci_activity":
		return "CI activity"
	case "state_change":
		return "state changed"
	case "comment":
		return "new comment"
	}
	return reason
}
