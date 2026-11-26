package gitui

import (
	"fmt"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	changePopupWidth  = 640
	changePopupHeight = 280
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
	t := c.Theme()
	ui.Overlay(c, func() {
		back := ui.Column(c).Absolute().Left(0).Top(0).Right(0).Bottom(0).AlignItems(ui.Center).Modal()
		back.Children(func() {
			panel := ui.Column(c).Margin(popupTop, 0, 0, 0).Width(changePopupWidth).Padding(popupPad).Gap(4).
				Background(t.Background).Radius(popupRadius).BorderWidth(1).BorderColor(t.Border).
				Role(ui.RoleDialog).Label("Change")
			panel.Children(func() {
				ui.Row(c).FillWidth().Gap(4).AlignItems(ui.Center).Children(func() {
					ui.Text(c, fmt.Sprintf("Change %d of %d", s.index+1, len(hunks))).FontSize(smallFont).Bold()
					if textButton(c, "Previous Change", s.index <= 0).Clicked() {
						s.index--
						s.diff.GoToHunk(s.index)
					}
					if textButton(c, "Next Change", s.index >= len(hunks)-1).Clicked() {
						s.index++
						s.diff.GoToHunk(s.index)
					}
					ui.Box(c).Grow(1)
					h := hunks[s.index]
					if textButton(c, "Copy Old Text", h.OldStart == h.OldEnd).Clicked() {
						c.WriteClipboard(s.diff.Old.Slice(lineOffset(s.diff.Old, h.OldStart), lineOffset(s.diff.Old, h.OldEnd)))
					}
					if textButton(c, "Show Diff", false).Clicked() {
						s.open = false
						if rel, ok := g.paths.rel(s.path); ok {
							g.openDiff(diffHead, rel)
						}
					}
					if textButton(c, "Rollback", false).Clicked() {
						rollbackHunk(s.diff.New, s.diff.Old, h)
						s.open = false
					}
				})
				editor.DiffView(c.Key("change"), s.diff).FillWidth().Height(changePopupHeight)
			})
			if panel.PressedOutside() || back.OverlayShortcut(0, ui.KeyEscape) {
				s.open = false
			}
		})
	})
}
