package gitui

import (
	"fmt"

	"github.com/ZerGo0/editor/internal/design"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

type changeState struct {
	open  bool
	path  string
	st    *editor.State
	diff  *editor.DiffState
	index int
}

func hunkAt(hunks []editor.DiffHunk, line int) int {
	for i, h := range hunks {
		if line >= h.NewStart && line < h.NewEnd {
			return i
		}
		if h.NewStart == h.NewEnd && (line == h.NewStart || line == h.NewStart-1) {
			return i
		}
	}
	return -1
}

func lineOffset(b *editor.Buffer, line int) int {
	if line >= b.LineCount() {
		return b.Len()
	}
	return b.Offset(editor.Pos{Line: line})
}

func (g *Git) activeChange() (*editor.State, string, *editor.DiffState, int, bool) {
	path := g.h.ActivePath()
	st := g.h.ActiveEditor()
	if !g.ready() || st == nil || st.Buffer == nil || path == "" {
		g.h.Notify("Open a file of this repository first.")
		return nil, "", nil, -1, false
	}
	base := g.Base(path)
	if base == nil {
		g.h.Notify("This file has no committed version to compare with yet.")
		return nil, "", nil, -1, false
	}
	d := &editor.DiffState{Old: base, New: st.Buffer, Layout: editor.DiffUnified}
	i := hunkAt(d.Hunks(), st.CursorPos().Line)
	if i < 0 {
		g.h.Notify("The caret is not on a changed line.")
		return nil, "", nil, -1, false
	}
	return st, path, d, i, true
}

func (g *Git) showChange() {
	st, path, d, i, ok := g.activeChange()
	if !ok {
		return
	}
	d.GoToHunk(i)
	g.change = changeState{open: true, path: path, st: st, diff: d, index: i}
}

func (g *Git) rollbackLines() {
	st, _, d, i, ok := g.activeChange()
	if !ok {
		return
	}
	rollbackHunk(st.Buffer, d.Old, d.Hunks()[i])
}

func rollbackHunk(buf, base *editor.Buffer, h editor.DiffHunk) {
	old := base.Slice(lineOffset(base, h.OldStart), lineOffset(base, h.OldEnd))
	buf.Edit(editor.Edit{Start: lineOffset(buf, h.NewStart), End: lineOffset(buf, h.NewEnd), Text: old})
}

func (g *Git) changePopup(c *ui.Context) {
	s := &g.change
	if !s.open {
		return
	}
	hunks := s.diff.Hunks()
	if s.index >= len(hunks) {
		s.open = false
		return
	}
	p := token.Of(c)
	ui.Overlay(c, func() {
		back := ui.Column(c).Absolute().Left(token.Space0.Px()).Top(token.Space0.Px()).Right(token.Space0.Px()).Bottom(token.Space0.Px()).
			AlignItems(ui.Center).Modal()
		back.Children(func() {
			panel := ui.Column(c).Margin(token.TitleBarHeight.Px()+token.OffsetQuickOpen.Px(), token.Space0.Px(), token.Space0.Px(), token.Space0.Px()).
				Width(token.DialogWidthWide.Px()).Padding(token.PadPopover.Px()).Gap(token.GapStack.Px()).
				Background(p.Layer.Widget).Radius(token.RadiusPopover.Px()).Border(token.BorderWidth.Px(), p.Border.Widget).
				ZIndex(token.LayerPopover.Z()).Role(ui.RoleDialog).Label("Change")
			token.ElevationPopover.Apply(panel, p)
			panel.Children(func() {
				ui.Row(c).FillWidth().Gap(token.GapTight.Px()).AlignItems(ui.Center).Children(func() {
					token.TextStrong.Apply(ui.Text(c, fmt.Sprintf("Change %d of %d", s.index+1, len(hunks)))).TextColor(p.Text.Primary).SingleLine()
					if iconButton(c, "Previous Change", icons.ChevronUp, s.index <= 0) {
						s.index--
						s.diff.GoToHunk(s.index)
					}
					if iconButton(c, "Next Change", icons.ChevronDown, s.index >= len(hunks)-1) {
						s.index++
						s.diff.GoToHunk(s.index)
					}
					design.Spacer(c)
					h := hunks[s.index]
					if iconButton(c, "Copy Old Text", icons.Copy, h.OldStart == h.OldEnd) {
						c.WriteClipboard(s.diff.Old.Slice(lineOffset(s.diff.Old, h.OldStart), lineOffset(s.diff.Old, h.OldEnd)))
					}
					if iconButton(c, "Show Diff", icons.Preview, false) {
						s.open = false
						if rel, ok := g.paths.rel(s.path); ok {
							g.openDiff(diffHead, rel)
						}
					}
					if iconButton(c, "Rollback", icons.Undo, false) {
						rollbackHunk(s.diff.New, s.diff.Old, h)
						s.open = false
					}
					if iconButton(c, "Close", icons.Close, false) {
						s.open = false
					}
				})
				editor.DiffView(c.Key("change"), s.diff).FillWidth().Height(token.PeekHeight.Px())
			})
			if panel.PressedOutside() || back.OverlayShortcut(0, ui.KeyEscape) {
				s.open = false
			}
		})
	})
}
