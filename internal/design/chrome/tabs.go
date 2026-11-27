package chrome

import (
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

const (
	closeLabel   = "Close"
	unpinLabel   = "Unpin"
	dirtyLabel   = "Unsaved changes"
	middleButton = 2
	noTab        = -1
)

type stripState struct {
	scroll ui.ScrollState
	middle int
}

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
	Label              string
	OnClose            func(i int)
	Menu               func(i int, m *ui.Menu)
	Actions            func(c *ui.Context)
	OnSelect           func(i int)
	OnUnpin            func(i int)
	OnAuxClick         func(i int)
	OnDoubleClick      func(i int)
	Drag               func(i int) any
	Drop               func(at int, target ui.Element) bool
	EmptyMenu          func(m *ui.Menu)
	OnEmptyDoubleClick func()
	Dragging           func(i int)
}

func TabStrip(c *ui.Context, selected *int, tabs []Tab, o TabStripOptions) ui.Element {
	p := token.Of(c)
	strip := ui.Row(c.Key("tab-strip:"+o.Label)).FillWidth().Height(token.TabHeight.Px()).Shrink(0).Background(p.Layer.TabBar).
		Padding(token.TabStripInset.Px(), 0, 0, token.GapInline.Px()).AlignItems(ui.Center)
	shown := ui.Local(strip, "shown", func() int { return noTab })
	state := ui.Local(strip, "state", func() stripState { return stripState{middle: noTab} })
	strip.Children(func() {
		ui.ScrollHorizontal(c).TrackScroll(&state.scroll).HandleInput(state.wheel).Shrink(1).MinWidth(0).FillHeight().Children(func() {
			parts := ui.TabsBase(c, selected, len(tabs))
			parts.List.FillHeight().AlignItems(ui.Center).Gap(token.GapTight.Px()).Label(o.Label).Children(func() {
				for i, t := range tabs {
					tab := parts.Tab(i)
					tabItem(c, p, tab, i == *selected, i, t, o, state)
					if i == *selected && *shown != i {
						tab.ScrollIntoView()
					}
				}
			})
			if parts.List.Changed() && o.OnSelect != nil {
				o.OnSelect(*selected)
			}
		})
		*shown = *selected
		rest := ui.Box(c.Key("rest")).Grow(1).FillHeight().MinWidth(token.TabMinWidth.Px() / 2)
		if o.Drop != nil && o.Drop(len(tabs), rest) {
			dropMark(c, p)
		}
		if o.EmptyMenu != nil {
			rest.ContextMenu(o.EmptyMenu)
		}
		if o.OnEmptyDoubleClick != nil && rest.DoubleClicked() {
			o.OnEmptyDoubleClick()
		}
		if o.Actions != nil {
			ui.Row(c).FillHeight().Padding(0, token.GapInline.Px()).Gap(token.GapTight.Px()).AlignItems(ui.Center).Shrink(0).
				Role(ui.RoleToolbar).Label(o.Label + " Actions").Children(func() { o.Actions(c) })
		}
	})
	return strip
}

func dropMark(c *ui.Context, p *token.Palette) {
	ui.Box(c).Absolute().Left(0).Top(token.GapInline.Px()).Bottom(token.GapInline.Px()).Width(token.Stroke2.Px()).
		Background(p.Accent.Base).PassThrough()
}

func (s *stripState) wheel(ev ui.InputEvent) bool {
	if ev.Kind != ui.InputScroll || ev.DY == 0 || ev.DX != 0 || ev.Mods != 0 {
		return false
	}
	s.scroll.X = max(0, min(s.scroll.X+ev.DY, s.scroll.MaxX))
	return true
}

func tabItem(c *ui.Context, p *token.Palette, tab ui.Element, active bool, i int, t Tab, o TabStripOptions, s *stripState) {
	hovered := tab.Hovered()
	fg := p.Tab.InactiveText
	switch {
	case active:
		tab.Background(p.List.InactiveSelection)
		fg = p.Tab.ActiveText
	case hovered:
		tab.Background(p.List.Hover)
	}
	if t.Git != token.GitNone {
		fg = p.GitColor(t.Git)
	}
	tab.Height(token.ControlHeight.Px()).MinWidth(token.TabMinWidth.Px()).MaxWidth(token.TabMaxWidth.Px()).
		Padding(0, token.PadInputEnd.Px(), 0, token.PadControlX.Px()).Radius(token.RadiusCard.Px()).
		Gap(token.GapRowIcon.Px()).AlignItems(ui.Center).Shrink(0).Label(t.Label).FocusRingOffset(-token.FocusRingWidth.Px())
	if t.Tooltip != "" {
		action.Tooltip(c, tab, t.Tooltip)
	}
	if t.Dirty {
		tab.Description(dirtyLabel)
	}
	if o.Menu != nil {
		tab.ContextMenu(func(m *ui.Menu) { o.Menu(i, m) })
	}
	if o.Drag != nil {
		if data := o.Drag(i); data != nil {
			tab.Drag(data)
		}
	}
	if o.Dragging != nil && tab.Dragging() {
		o.Dragging(i)
	}
	tab.Children(func() {
		if o.Drop != nil && o.Drop(i, tab) {
			dropMark(c, p)
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
		tabEnd(c, p, active, hovered, i, t, o)
	})
	if b, ok := tab.PressedButton(); ok && b == middleButton {
		s.middle = i
	} else if s.middle == i {
		s.middle = noTab
		if hovered && o.OnAuxClick != nil {
			o.OnAuxClick(i)
		}
	}
	if o.OnDoubleClick != nil && tab.DoubleClicked() {
		o.OnDoubleClick(i)
	}
}

func tabEnd(c *ui.Context, p *token.Palette, active, hovered bool, i int, t Tab, o TabStripOptions) {
	slot := ui.Box(c).Size(token.IconButtonSize.Px(), token.IconButtonSize.Px()).Center().Shrink(0)
	slot.Children(func() {
		pointed := hovered || slot.Hovered()
		switch {
		case t.Dirty && t.Pinned && !pointed:
			ui.Icon(c, icons.Pin).Size(token.IconSizeSmall.Px(), token.IconSizeSmall.Px()).TextColor(p.Icon).AccessHidden(true)
			ui.Box(c).Absolute().Right(token.GapInline.Px()).Bottom(token.GapInline.Px()).Size(token.PinnedDirtyDotSize.Px(), token.PinnedDirtyDotSize.Px()).
				Radius(token.RadiusPill.Px()).Background(p.Text.Primary).AccessHidden(true)
		case t.Dirty && !pointed:
			ui.Box(c).Size(token.DirtyDotSize.Px(), token.DirtyDotSize.Px()).Radius(token.RadiusPill.Px()).Background(p.Text.Primary).
				AccessHidden(true)
		case t.Pinned && o.OnUnpin != nil:
			if action.IconButton(c, icons.Pin, unpinLabel+" "+t.Label, action.IconOptions{Tooltip: unpinLabel}).Clicked() {
				o.OnUnpin(i)
			}
		case t.Pinned:
			ui.Icon(c, icons.Pin).Size(token.IconSizeSmall.Px(), token.IconSizeSmall.Px()).TextColor(p.Icon).AccessHidden(true)
		case pointed || active:
			if action.IconButton(c, icons.Close, closeLabel+" "+t.Label, action.IconOptions{Tooltip: closeLabel}).Clicked() && o.OnClose != nil {
				o.OnClose(i)
			}
		}
	})
}
