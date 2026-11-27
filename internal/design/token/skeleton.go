package token

import "github.com/egoist/mygo/ui"

const skeletonInk float32 = 0.1

func (p *Palette) IsSkeleton() bool { return p.skeleton }

func (p *Palette) Skeleton() *Palette {
	registry.Lock()
	sk, ok := registry.skeletons[p]
	registry.Unlock()
	if ok {
		return sk
	}
	s := *p
	s.skeleton = true
	ink := p.Layer.SideBar.Mix(p.Text.Primary, skeletonInk)
	s.Text = Texts{Primary: ink, Strong: ink, Secondary: ink, Muted: ink, Disabled: ink, Link: ink, Selection: p.Text.Selection}
	s.Icon = ink
	s.Accent.Base, s.Accent.Text, s.Accent.Hover = ink, ink, ink
	s.Badge = Badges{Background: ink, Text: ui.Transparent, Accent: ink, AccentText: ui.Transparent}
	s.Git = GitColors{Added: ink, Modified: ink, Deleted: ink, Renamed: ink, Untracked: ink, Ignored: ink, Conflict: ink, Submodule: ink}
	s.Severity.Error, s.Severity.Warning, s.Severity.Success = ink, ink, ink
	s.Bar.ActivityActive, s.Bar.ActivityInactive = ink, ink
	s.Tab.ActiveText, s.Tab.InactiveText = ink, ink
	s.List.ActiveSelectionText, s.List.ActiveSelectionSecondary, s.List.PickSelectionText = ink, ink, ink
	for i := range s.Avatar {
		s.Avatar[i] = ink
	}
	s.OnAvatar = ui.Transparent
	registry.Lock()
	defer registry.Unlock()
	if sk, ok := registry.skeletons[p]; ok {
		return sk
	}
	registry.skeletons[p] = &s
	return &s
}
