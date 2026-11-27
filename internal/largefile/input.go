package largefile

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
)

const copyLimit = 64 << 20

func (s *State) input(ev ui.InputEvent) bool {
	switch ev.Kind {
	case ui.InputKeyDown:
		s.notice = ""
		return s.key(ev.Mods, ev.Key)
	case ui.InputText:
		text := strings.Map(func(r rune) rune {
			if r < ' ' || r == asciiDelete {
				return -1
			}
			return r
		}, ev.Text)
		if text == "" {
			return false
		}
		s.compose = ""
		s.typeText(text)
		return true
	case ui.InputCompose:
		s.compose = ev.Text
		return true
	case ui.InputCommand:
		return s.command(ev.Text)
	case ui.InputPointerDown:
		return s.pointerDown(ev)
	case ui.InputPointerMove:
		return s.pointerMove(ev)
	case ui.InputPointerUp:
		held := s.dragging || s.bar.Active
		s.dragging, s.bar.Active = false, false
		return held
	case ui.InputScroll:
		s.wheel(ev.DX, ev.DY)
		return true
	}
	return false
}

func (s *State) key(mods ui.Modifiers, key ui.Key) bool {
	extend := mods&ui.Shift != 0
	d := s.Doc
	switch mods &^ ui.Shift {
	case 0:
		switch key {
		case ui.KeyLeft:
			s.moveH(-1, extend)
		case ui.KeyRight:
			s.moveH(1, extend)
		case ui.KeyUp:
			s.moveV(-1, extend)
		case ui.KeyDown:
			s.moveV(1, extend)
		case ui.KeyHome:
			s.moveTo(lineStartOf(d, s.Caret), extend)
		case ui.KeyEnd:
			s.moveTo(lineEndOf(d, s.Caret), extend)
		case ui.KeyPageUp:
			s.page(-1, extend)
		case ui.KeyPageDown:
			s.page(1, extend)
		case ui.KeyBackspace:
			s.deleteBy(prevBoundary)
		case ui.KeyDelete:
			s.deleteBy(nextBoundary)
		case ui.KeyEnter:
			s.typeText(s.lineEnding())
		case ui.KeyTab:
			if extend {
				return false
			}
			s.typeText("\t")
		case ui.KeyEscape:
			return s.escape()
		default:
			return false
		}
		return true
	case ui.Cmd:
		switch key {
		case ui.KeyLeft:
			s.moveTo(lineStartOf(d, s.Caret), extend)
		case ui.KeyRight:
			s.moveTo(lineEndOf(d, s.Caret), extend)
		case ui.KeyUp, ui.KeyHome:
			s.moveTo(0, extend)
		case ui.KeyDown, ui.KeyEnd:
			s.moveTo(d.Len(), extend)
		case ui.KeyA:
			s.command("selectAll")
		case ui.KeyC:
			s.command("copy")
		case ui.KeyX:
			s.command("cut")
		case ui.KeyV:
			s.command("paste")
		case ui.KeyZ:
			if extend {
				s.command("redo")
			} else {
				s.command("undo")
			}
		case ui.KeyF:
			s.openFind()
		case ui.KeyG:
			s.search(extend)
		default:
			return false
		}
		return true
	}
	return false
}

func (s *State) command(name string) bool {
	d := s.Doc
	switch name {
	case "selectAll":
		s.Anchor, s.Caret = 0, d.Len()
		s.hasGoal = false
	case "copy":
		s.copySelection()
	case "cut":
		if s.copySelection() {
			s.typeText("")
		}
	case "paste", "pasteAndMatchStyle":
		text := s.services.ReadClipboard()
		if len(text) > copyLimit {
			s.notice = fmt.Sprintf("Clipboard text is larger than %d MB", copyLimit>>20)
			return true
		}
		s.typeText(text)
	case "undo":
		if start, end, ok := d.Undo(); ok {
			s.Anchor, s.Caret = start, end
			s.hasGoal = false
			s.reveal()
		}
	case "redo":
		if _, end, ok := d.Redo(); ok {
			s.Anchor, s.Caret = end, end
			s.hasGoal = false
			s.reveal()
		}
	case "delete":
		s.typeText("")
	default:
		return false
	}
	return true
}

func (s *State) copySelection() bool {
	a, b := s.selection()
	if a == b {
		return false
	}
	if b-a > copyLimit {
		s.notice = fmt.Sprintf("Selection is larger than %d MB and was not copied", copyLimit>>20)
		return false
	}
	text, err := s.Doc.Slice(a, b)
	if err != nil {
		s.notice = err.Error()
		return false
	}
	s.services.WriteClipboard(text)
	return true
}

func (s *State) typeText(text string) {
	a, b := s.selection()
	if a == b && text == "" {
		return
	}
	s.Doc.Replace(a, b, text)
	s.Caret = a + int64(len(text))
	s.Anchor = s.Caret
	s.hasGoal = false
	s.reveal()
}

func (s *State) deleteBy(step func(*Doc, int64) int64) {
	if a, b := s.selection(); a == b {
		s.Anchor = step(s.Doc, s.Caret)
	}
	s.typeText("")
}

func (s *State) lineEnding() string {
	end := lineEndOf(s.Doc, s.Caret)
	if peek(s.Doc, end, end+crlfLen) == "\r\n" {
		return "\r\n"
	}
	return "\n"
}

func (s *State) escape() bool {
	if s.find.open {
		s.closeFind()
		return true
	}
	if s.Caret != s.Anchor {
		s.Anchor = s.Caret
		return true
	}
	return false
}

func (s *State) moveTo(off int64, extend bool) {
	s.Doc.breakGroup()
	s.Caret = clamp(off, 0, s.Doc.Len())
	if !extend {
		s.Anchor = s.Caret
	}
	s.hasGoal = false
	s.reveal()
}

func (s *State) moveH(dir int, extend bool) {
	a, b := s.selection()
	switch {
	case !extend && a != b && dir < 0:
		s.moveTo(a, false)
	case !extend && a != b:
		s.moveTo(b, false)
	case dir < 0:
		s.moveTo(prevBoundary(s.Doc, s.Caret), extend)
	default:
		s.moveTo(nextBoundary(s.Doc, s.Caret), extend)
	}
}

func (s *State) moveV(n int, extend bool) {
	d := s.Doc
	d.breakGroup()
	if !s.hasGoal {
		s.goalX, s.hasGoal = s.caretX(), true
	}
	target := lineStartOf(d, s.Caret)
	moved := 0
	for ; moved < abs(n); moved++ {
		if n < 0 {
			if target == 0 {
				break
			}
			target = lineStartOf(d, target-1)
			continue
		}
		next, ok := nextLineStart(d, target)
		if !ok {
			break
		}
		target = next
	}
	var off int64
	switch {
	case moved > 0:
		li, err := readLine(d, target)
		if err != nil {
			s.notice = err.Error()
			return
		}
		off = target + int64(s.shape(li.raw).at(s.goalX))
	case n < 0:
		off = 0
	default:
		off = d.Len()
	}
	s.Caret = off
	if !extend {
		s.Anchor = off
	}
	s.reveal()
}

func abs(n int) int { return max(n, -n) }

func (s *State) page(dir int, extend bool) {
	n := dir * max(1, s.rows()-1)
	s.scrollLines(n)
	s.moveV(n, extend)
}

func (s *State) caretX() float32 {
	start := lineStartOf(s.Doc, s.Caret)
	li, err := readLine(s.Doc, start)
	if err != nil {
		return 0
	}
	return s.shape(li.raw).x(int(s.Caret - start))
}

func (s *State) reveal() {
	d := s.Doc
	ls := lineStartOf(d, s.Caret)
	rows := s.rows()
	if ls < s.Top {
		s.Top = ls
	} else if !s.lineVisible(ls, rows) {
		top := ls
		for i := 1; i < rows && top > 0; i++ {
			top = lineStartOf(d, top-1)
		}
		s.Top = top
	}
	x := s.caretX()
	switch w := s.textW(); {
	case x < s.scrollX:
		s.scrollX = max(0, x-revealMargin)
	case x > s.scrollX+w-revealMargin:
		s.scrollX = x - w + revealMargin
	}
}

func (s *State) lineVisible(ls int64, rows int) bool {
	if fr := s.frame; fr.version == s.Doc.Version() && fr.top == s.Top {
		for _, ln := range fr.lines[:min(rows, len(fr.lines))] {
			if ln.start == ls {
				return true
			}
		}
		if len(fr.lines) > rows {
			return false
		}
	}
	start := s.Top
	for range rows {
		if start == ls {
			return true
		}
		next, ok := nextLineStart(s.Doc, start)
		if !ok || next > ls {
			return false
		}
		start = next
	}
	return false
}

func (s *State) scrollLines(n int) {
	d := s.Doc
	if count, exact := d.LineCount(); exact {
		if l, ok := d.LineOf(s.Top); ok {
			if st, ok := d.LineStart(clamp(l+int64(n), 0, count-1)); ok {
				s.Top = st
			}
			return
		}
	}
	for ; n > 0; n-- {
		next, ok := nextLineStart(d, s.Top)
		if !ok {
			break
		}
		s.Top = next
	}
	for ; n < 0 && s.Top > 0; n++ {
		s.Top = lineStartOf(d, s.Top-1)
	}
}

func (s *State) wheel(dx, dy float32) {
	s.wheelY += dy
	if n := int(s.wheelY / s.lineH); n != 0 {
		s.scrollLines(n)
		s.wheelY -= float32(n) * s.lineH
	}
	s.scrollX = max(0, min(s.scrollX+dx, s.frame.maxW-s.textW()))
}

func (s *State) pointerDown(ev ui.InputEvent) bool {
	if ev.Button != 0 {
		return false
	}
	s.notice = ""
	if ev.X >= s.viewW-s.barStyle.TrackWidth {
		if bar, lines := s.scrollbar(); bar.Scrolls() {
			if offset, jumped := s.bar.Press(bar, ev.Y); jumped {
				s.scrollToBar(offset, lines)
			}
			return true
		}
	}
	off := s.hit(ev.X, ev.Y)
	s.Doc.breakGroup()
	s.Caret = off
	if ev.Mods&ui.Shift == 0 {
		s.Anchor = off
	}
	s.hasGoal = false
	s.dragging = true
	return true
}

func (s *State) pointerMove(ev ui.InputEvent) bool {
	switch {
	case s.bar.Active:
		bar, lines := s.scrollbar()
		s.scrollToBar(s.bar.Offset(bar, ev.Y), lines)
	case s.dragging && ev.Button == 0:
		s.Caret = s.hit(ev.X, ev.Y)
	default:
		return false
	}
	return true
}

func (s *State) scrollToBar(off float32, lines bool) {
	if lines {
		if st, ok := s.Doc.LineStart(int64(off)); ok {
			s.Top = st
		}
		return
	}
	s.Top = lineStartOf(s.Doc, int64(off))
}

func (s *State) hit(x, y float32) int64 {
	lines := s.frame.lines
	if len(lines) == 0 {
		return 0
	}
	row := int(y / s.lineH)
	if row >= len(lines) {
		last := lines[len(lines)-1]
		if !last.hasNext() {
			return max(last.end, last.start)
		}
		row = len(lines) - 1
	}
	ln := lines[max(0, row)]
	off := ln.start + int64(ln.shape.at(x-s.textX()))
	if ln.end >= ln.start {
		off = min(off, ln.end)
	}
	return off
}
