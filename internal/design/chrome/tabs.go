package chrome

import (
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

const (
	closeLabel = "Close"
	dirtyLabel = "Unsaved changes"
)

type Tab struct {
	Label       string
	Description string
	Tooltip     string
	Icon        *ui.SVG
	Leading     func(c *ui.Context)
	Git         token.GitStatus
	Dirty       bool
	Preview     bool
	Pinned      bool
}

type TabStripOptions struct {
	Label   string
	OnClose func(i int)
	Menu    func(i int, m *ui.Menu)
	Actions func(c *ui.Context)
}

func TabStrip(c *ui.Context, selected *int, tabs []Tab, o TabStripOptions) ui.Element {
	p := token.Of(c)
	strip := ui.Row(c.Key("tab-strip:" + o.Label)).FillWidth().Height(token.TabHeight.Px()).Shrink(0).Background(p.Layer.TabBar)
	strip.Children(func() {
		parts := ui.TabsBase(c, selected, len(tabs))
		parts.List.Grow(1).MinWidth(0).FillHeight().Label(o.Label).Children(func() {
			for i, t := range tabs {
				tabItem(c, p, parts.Tab(i), i == *selected, i, t, o)
			}
		})
		if o.Actions != nil {
			ui.Row(c).FillHeight().Padding(0, token.PadRow.Px()).Gap(token.GapTight.Px()).AlignItems(ui.Center).Shrink(0).
				Role(ui.RoleToolbar).Label(o.Label + " Actions").Children(func() { o.Actions(c) })
		}
	})
	return strip
}

func tabItem(c *ui.Context, p *token.Palette, tab ui.Element, active bool, i int, t Tab, o TabStripOptions) {
	hovered := tab.Hovered()
	bg, fg := p.Tab.Inactive, p.Tab.InactiveText
	switch {
	case active:
		bg, fg = p.Tab.Active, p.Tab.ActiveText
	case hovered:
		bg = p.Tab.Hover
	}
	if t.Git != token.GitNone {
		fg = p.GitColor(t.Git)
	}
	tab.FillHeight().MinWidth(token.TabMinWidth.Px()).Padding(0, token.PadInputEnd.Px(), 0, token.PadTabX.Px()).
		Gap(token.GapRowIcon.Px()).AlignItems(ui.Center).Shrink(0).Background(bg).Label(t.Label).
		BorderWidth(0, token.BorderWidth.Px(), 0, 0).BorderColor(p.Tab.Border).FocusRingOffset(-token.FocusRingWidth.Px())
	if t.Tooltip != "" {
		tab.Tooltip(t.Tooltip)
	}
	if t.Dirty {
		tab.Description(dirtyLabel)
	}
	if o.Menu != nil {
		tab.ContextMenu(func(m *ui.Menu) { o.Menu(i, m) })
	}
	tab.Children(func() {
		if active {
			ui.Box(c).Absolute().Top(0).Left(0).Right(0).Height(token.IndicatorWidth.Px()).Background(p.Tab.ActiveBorderTop)
		}
		switch {
		case t.Leading != nil:
			t.Leading(c)
		case t.Icon != nil:
			ui.Icon(c, t.Icon).Size(token.IconSize.Px(), token.IconSize.Px()).TextColor(p.Icon).Shrink(0).AccessHidden(true)
		}
		text := token.TextBody.Apply(ui.Text(c, t.Label)).TextColor(fg).SingleLine().MinWidth(0).Shrink(1).AccessHidden(true)
		if t.Preview {
			text.Italic()
		}
		if t.Description != "" {
			token.TextLabel.Apply(ui.Text(c, t.Description)).TextColor(p.Text.Secondary).SingleLine().MinWidth(0).Shrink(1).AccessHidden(true)
		}
		tabEnd(c, p, active, hovered, t, func() {
			if o.OnClose != nil {
				o.OnClose(i)
			}
		})
	})
}

func tabEnd(c *ui.Context, p *token.Palette, active, hovered bool, t Tab, onClose func()) {
	slot := ui.Box(c).Size(token.IconButtonSize.Px(), token.IconButtonSize.Px()).Center().Shrink(0)
	slot.Children(func() {
		switch {
		case t.Pinned:
			ui.Icon(c, icons.Pin).Size(token.IconSizeSmall.Px(), token.IconSizeSmall.Px()).TextColor(p.Icon).AccessHidden(true)
		case active || hovered:
			if action.IconButton(c, icons.Close, closeLabel+" "+t.Label, action.IconOptions{}).Clicked() {
				onClose()
			}
		case t.Dirty:
			ui.Box(c).Size(token.DirtyDotSize.Px(), token.DirtyDotSize.Px()).Radius(token.RadiusPill.Px()).Background(p.Text.Primary).
				AccessHidden(true)
		}
	})
}
