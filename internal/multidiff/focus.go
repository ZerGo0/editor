package multidiff

import "github.com/egoist/mygo/ui/editor"

type focusState struct {
	wanted bool
	key    string
	within bool
	caret  string
}

func (s *focusState) claim(key string, d *editor.DiffState) {
	if !s.wanted || s.key != "" && s.key != key {
		return
	}
	d.Focus()
	s.wanted, s.key = false, ""
}

func (v *View) Focus() { v.focus = focusState{wanted: true} }

func (v *View) Focused() bool { return v.focus.within }

func (v *View) Step(delta int) { v.step(delta) }

func (v *View) Caret() (path string, line int, ok bool) {
	if f := v.files[v.focus.caret]; f != nil && f.state != nil {
		return f.entry.Path, NewSideLine(f.state), true
	}
	if f := v.files[v.nav.key]; f != nil && f.state != nil {
		if hunks := f.state.Hunks(); v.nav.hunk >= 0 && v.nav.hunk < len(hunks) {
			return f.entry.Path, hunks[v.nav.hunk].NewStart, true
		}
	}
	if p, found := v.CurrentPath(); found {
		return p, -1, true
	}
	return "", -1, false
}

func NewSideLine(d *editor.DiffState) int {
	line := d.CursorPos().Line
	if d.ActiveSide() == editor.DiffNew {
		return line
	}
	return newLineOf(d.Hunks(), line)
}

func newLineOf(hunks []editor.DiffHunk, line int) int {
	shift := 0
	for _, h := range hunks {
		if line < h.OldStart {
			break
		}
		if line < h.OldEnd {
			if h.NewEnd <= h.NewStart {
				return h.NewStart
			}
			return min(h.NewStart+line-h.OldStart, h.NewEnd-1)
		}
		shift = h.NewEnd - h.OldEnd
	}
	return max(line+shift, 0)
}
