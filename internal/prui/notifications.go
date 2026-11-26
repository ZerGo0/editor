package prui

import (
	"fmt"
	"strconv"
	"time"

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
	if p.closed || p.phase != phaseReady {
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
	go func() {
		page, err := be.Notifications(p.ctx, repo, modified)
		p.post(func() {
			if gen != p.notifs.gen {
				return
			}
			ns := &p.notifs
			ns.loading = false
			ns.fetchedAt = p.env.Now()
			if err != nil {
				ns.err = err
				p.authFailure(err)
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
	}()
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
	go func() {
		err := be.MarkNotification(p.ctx, id, done)
		p.post(func() {
			if err != nil {
				p.h.ShowError("Cannot update the notification", err)
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
	}()
}

func (p *PRs) openNotification(n Notification) {
	p.openPR(n.Number, nil)
	if n.Unread {
		p.markNotification(n, false)
	}
}

func (p *PRs) NotificationsView(c *ui.Context) {
	if p.closed {
		return
	}
	if p.phase != phaseReady {
		p.stateView(c)
		return
	}
	p.notificationsDue(c)
	ns := &p.notifs
	ui.Column(c).Grow(1).FillWidth().MinHeight(0).Children(func() {
		ui.Row(c).Gap(spacingSmall).AlignItems(ui.Center).Padding(0, toolbarPad, toolbarPad, toolbarPad).Shrink(0).FillWidth().Children(func() {
			mutedText(c, "Pull request notifications of this repository").Grow(1).SingleLine().MinWidth(0)
			if ns.loading {
				ui.Spinner(c).Label("Loading notifications")
			}
			if iconButton(c, "refresh-notifications", "Refresh notifications", icons.Refresh).Clicked() {
				ns.fetchedAt = time.Time{}
				ns.modified = ""
			}
		})
		switch {
		case ns.err != nil && len(ns.items) == 0:
			errorBlock(c, ns.err, func() { ns.fetchedAt = time.Time{} })
		case !ns.loaded:
			loadingRow(c, "Loading notifications...")
		case len(ns.items) == 0:
			mutedText(c, "You are all caught up. New pull request activity shows here.").Padding(rowPadY, rowPadX)
		default:
			ns.list.Selected = &ns.row
			ns.list.Key = func(i int) any { return ns.items[i].ID }
			ns.list.Label = func(i int) string {
				return fmt.Sprintf("%s, #%d, %s", ns.items[i].Title, ns.items[i].Number, ns.items[i].Reason)
			}
			list := ui.List(c, &ns.list, len(ns.items), func(i int) { p.notificationRow(c, ns.items[i]) }).Grow(1).FillWidth().MinHeight(0).Label("Notifications")
			if list.Submitted() && ns.row >= 0 && ns.row < len(ns.items) {
				p.openNotification(ns.items[ns.row])
			}
		}
	})
}

func (p *PRs) notificationRow(c *ui.Context, n Notification) {
	t := c.Theme()
	now := p.env.Now()
	row := ui.Row(c).Gap(spacingMedium).AlignItems(ui.Start).Padding(rowPadY, rowPadX).FillWidth()
	row.Children(func() {
		dot := ui.Icon(c, icons.Dot).Size(iconSmall, iconSmall).Shrink(0).Margin(prRowGap, 0, 0, 0)
		if n.Unread {
			dot.TextColor(t.Accent).Label("Unread")
		} else {
			dot.TextColor(ui.Transparent)
		}
		ui.Column(c).Grow(1).MinWidth(0).Gap(prRowGap).Children(func() {
			title := ui.Text(c, n.Title).FontSize(fontBody).SingleLine()
			if n.Unread {
				title.Bold()
			}
			ui.Row(c).Gap(spacingSmall + prRowGap).AlignItems(ui.Center).Children(func() {
				mutedText(c, "#"+strconv.Itoa(n.Number)).SingleLine().Shrink(0)
				mutedText(c, reasonLabel(n.Reason)).SingleLine().Shrink(1).MinWidth(0)
				mutedText(c, relativeTime(now, n.Updated)).SingleLine().Shrink(0)
			})
		})
	})
	if row.RowClicked() {
		p.openNotification(n)
	}
	row.ContextMenu(func(m *ui.Menu) {
		if m.Item("Open Pull Request").Chosen() {
			p.openNotification(n)
		}
		if m.Item("Open on GitHub").Chosen() {
			openExternal(c, pullURL(p.repo, n.Number))
		}
		m.Separator()
		if m.Item("Mark as Read").Disabled(!n.Unread).Chosen() {
			p.markNotification(n, false)
		}
		if m.Item("Mark as Done").Chosen() {
			p.markNotification(n, true)
		}
	})
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
