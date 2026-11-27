package largefile

import (
	"fmt"
	"math"
	"strconv"

	"github.com/ZerGo0/editor/internal/design"
	"github.com/ZerGo0/editor/internal/design/scrollbar"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

const (
	fontFamily       = "monospace"
	fontSize         = 13
	lineHeightFactor = 1.5
	gutterPad        = 12
	minGutterDigits  = 3
	textPad          = 8
	caretWidth       = 2
	revealMargin     = 24
	defaultViewW     = 800
	defaultViewH     = 600
	shapeCacheMax    = 512
	percent          = 100
	unknownLine      = "?"
	cutMarker        = " ..."
	editorLabel      = "Large file editor"
	measureSample    = "0"
	newlineMarkRatio = 0.5
)

type State struct {
	Doc    *Doc
	Caret  int64
	Anchor int64
	Top    int64

	scrollX  float32
	goalX    float32
	hasGoal  bool
	viewW    float32
	viewH    float32
	wheelY   float32
	compose  string
	notice   string
	dragging bool
	focused  bool
	bar      scrollbar.Drag
	barStyle ui.ScrollbarStyle
	lineH    float32
	charW    float32
	focus    ui.Handle
	services ui.Services
	watching *Doc
	shapes   map[string]*shaped
	frame    frame
	find     finder
}

type shaped struct {
	disp   display
	layout *ui.TextLayout
	w, h   float32
}

type viewLine struct {
	lineInfo
	shape *shaped
}

type frame struct {
	version  uint64
	top      int64
	topLine  int64
	numbered bool
	lines    []viewLine
	gutterW  float32
	maxW     float32
}

func (s *State) Handle() *ui.Handle {
	return &s.focus
}

func View(c *ui.Context, s *State) ui.Element {
	if s.Doc == nil {
		return ui.Box(c).Grow(1)
	}
	pal := token.Of(c)
	s.services = c.Services()
	s.barStyle = token.ScrollbarStyle(pal)
	s.init()
	s.applyFound()
	col := ui.Column(c).Grow(1)
	col.Children(func() {
		if s.find.open {
			s.findBar(c)
		}
		s.textArea(c, pal)
		s.statusLine(c)
	})
	return col
}

func (s *State) init() {
	if s.watching != s.Doc {
		s.watching = s.Doc
		s.Doc.OnIndexProgress(s.services.Invalidate)
	}
	if s.shapes == nil {
		s.shapes = map[string]*shaped{}
	}
	if s.lineH == 0 {
		s.lineH = float32(math.Round(fontSize * lineHeightFactor))
		s.charW = s.shape(measureSample).w
	}
	if s.viewW == 0 {
		s.viewW, s.viewH = defaultViewW, defaultViewH
	}
}

func font() ui.Font { return ui.Font{Family: fontFamily, Size: fontSize} }

func (s *State) shape(raw string) *shaped {
	if sh, ok := s.shapes[raw]; ok {
		return sh
	}
	if len(s.shapes) >= shapeCacheMax {
		clear(s.shapes)
	}
	disp := newDisplay(raw, false)
	l := ui.ShapeText(disp.text, font(), 0)
	w, h := l.Size()
	sh := &shaped{disp: disp, layout: l, w: w, h: h}
	s.shapes[raw] = sh
	return sh
}

// TECHDEBT: A caret past the display cut of a long line shows at the cut marker; page the line horizontally if editing past maxDisplayRunes matters.
func (sh *shaped) x(off int) float32 {
	if off >= sh.disp.shown() {
		return sh.w
	}
	return sh.layout.Caret(sh.disp.unitOf(off)).X
}

func (sh *shaped) at(x float32) int {
	return sh.disp.byteOf(sh.layout.IndexAt(ui.Point{X: x, Y: sh.h / 2}))
}

func (s *State) rows() int {
	return max(1, int(s.viewH/s.lineH))
}

func (s *State) textX() float32 { return s.frame.gutterW + textPad - s.scrollX }

func (s *State) textW() float32 {
	return max(0, s.viewW-s.frame.gutterW-textPad-s.barStyle.TrackWidth)
}

func (s *State) layout() {
	d := s.Doc
	s.Top = lineStartOf(d, s.Top)
	fr := frame{version: d.Version(), top: s.Top}
	fr.topLine, fr.numbered = d.LineOf(s.Top)
	start := s.Top
	for len(fr.lines) <= s.rows() {
		li, err := readLine(d, start)
		if err != nil {
			s.notice = err.Error()
			break
		}
		sh := s.shape(li.raw)
		li.cut = li.cut || sh.disp.cut
		fr.lines = append(fr.lines, viewLine{lineInfo: li, shape: sh})
		fr.maxW = max(fr.maxW, sh.w)
		if !li.hasNext() {
			break
		}
		start = li.next
	}
	digits := max(minGutterDigits, len(strconv.FormatInt(fr.topLine+int64(len(fr.lines)), 10)))
	fr.gutterW = float32(digits)*s.charW + 2*gutterPad
	fr.maxW += s.charW
	s.frame = fr
}

func (l viewLine) contains(off int64) bool {
	return off >= l.start && (!l.hasNext() || off < l.next)
}

func (s *State) caretRow() (int, float32, bool) {
	for i, ln := range s.frame.lines {
		if ln.contains(s.Caret) {
			return i, ln.shape.x(int(s.Caret - ln.start)), true
		}
	}
	return 0, 0, false
}

func (s *State) caretRect() ui.Rect {
	row, x, ok := s.caretRow()
	if !ok {
		return ui.Rect{X: s.frame.gutterW + textPad, W: caretWidth, H: s.lineH}
	}
	return ui.Rect{X: s.textX() + x, Y: float32(row) * s.lineH, W: caretWidth, H: s.lineH}
}

func (s *State) selection() (int64, int64) {
	return min(s.Caret, s.Anchor), max(s.Caret, s.Anchor)
}

func (s *State) textArea(c *ui.Context, t *token.Palette) {
	s.layout()
	e := ui.Box(c).Grow(1).FillWidth().Focusable().FocusRing(false).Clip().Cursor(ui.CursorText).
		Role(ui.RoleTextField).Label(editorLabel).Bind(&s.focus)
	s.focused = e.Focused()
	e.HandleInput(s.input).TextCaret(s.caretRect()).Draw(func(p *ui.Painter, r ui.Rect) { s.paint(p, r, t) })
}

func (s *State) paint(p *ui.Painter, r ui.Rect, t *token.Palette) {
	if r.W != s.viewW || r.H != s.viewH {
		s.viewW, s.viewH = r.W, r.H
		s.services.Invalidate()
	}
	fr := &s.frame
	p.Fill(r, t.Layer.Editor, 0)
	x0 := r.X + s.textX()
	a, b := s.selection()
	p.Clip(ui.Rect{X: r.X + fr.gutterW, Y: r.Y, W: max(0, r.W-fr.gutterW), H: r.H}, 0, func() {
		for i, ln := range fr.lines {
			y := r.Y + float32(i)*s.lineH
			if r, ok := s.selectionRect(ln, x0, y, a, b); a < b && ok {
				p.Fill(r, t.Text.Selection, 0)
			}
			ty := y + (s.lineH-ln.shape.h)/2
			p.TextLayout(ln.shape.layout, x0, ty, t.Text.Primary)
			if ln.cut {
				m := s.shape(cutMarker)
				p.TextLayout(m.layout, x0+ln.shape.w, ty, t.Text.Muted)
			}
		}
		if row, x, ok := s.caretRow(); ok && s.focused {
			y := r.Y + float32(row)*s.lineH
			if s.compose != "" {
				s.paintCompose(p, x0+x, y, t)
			} else {
				p.Fill(ui.Rect{X: x0 + x, Y: y, W: caretWidth, H: s.lineH}, t.Accent.Base, 0)
			}
		}
	})
	p.Fill(ui.Rect{X: r.X, Y: r.Y, W: fr.gutterW, H: r.H}, t.Layer.Editor, 0)
	caretRow, _, caretShown := s.caretRow()
	for i := range fr.lines {
		label := unknownLine
		if fr.numbered {
			label = strconv.FormatInt(fr.topLine+int64(i)+1, 10)
		}
		color := t.Text.Muted
		if caretShown && i == caretRow {
			color = t.Text.Primary
		}
		sh := s.shape(label)
		y := r.Y + float32(i)*s.lineH + (s.lineH-sh.h)/2
		p.TextLayout(sh.layout, r.X+fr.gutterW-gutterPad-sh.w, y, color)
	}
	bar, _ := s.scrollbar()
	bar.Box.X, bar.Box.Y = r.X, r.Y
	if bar.Scrolls() {
		p.Scrollbar(bar, ui.ScrollbarState{Dragging: s.bar.Active})
	}
}

func (s *State) selectionRect(ln viewLine, x0, y float32, a, b int64) (ui.Rect, bool) {
	end := ln.end
	if end < ln.start {
		end = ln.start + int64(ln.shape.disp.shown())
	}
	if b <= ln.start || a > end || (ln.hasNext() && a >= ln.next) {
		return ui.Rect{}, false
	}
	x1 := ln.shape.x(int(max(a, ln.start) - ln.start))
	x2 := ln.shape.x(int(min(b, end) - ln.start))
	if b > end && ln.hasNext() {
		x2 += s.charW * newlineMarkRatio
	}
	return ui.Rect{X: x0 + x1, Y: y, W: x2 - x1, H: s.lineH}, x2 > x1
}

func (s *State) paintCompose(p *ui.Painter, x, y float32, t *token.Palette) {
	sh := s.shape(s.compose)
	p.Fill(ui.Rect{X: x, Y: y, W: sh.w, H: s.lineH}, t.Layer.Input, 0)
	p.TextLayout(sh.layout, x, y+(s.lineH-sh.h)/2, t.Text.Primary)
	p.Line(x, y+s.lineH-1, x+sh.w, y+s.lineH-1, token.BorderWidth.Px(), t.Accent.Base)
}

func (s *State) scrollbar() (ui.Scrollbar, bool) {
	rows := float32(s.rows())
	bar := ui.Scrollbar{Box: ui.Rect{W: s.viewW, H: s.viewH}, Style: s.barStyle}
	if n, exact := s.Doc.LineCount(); exact && s.frame.numbered {
		bar.View, bar.Content, bar.Offset = rows, float32(n-1)+rows, float32(s.frame.topLine)
		return bar, true
	}
	view := float32(1)
	if k := len(s.frame.lines); k > 0 {
		last := s.frame.lines[k-1]
		view = float32(max(1, max(last.next, last.end)-s.frame.top))
	}
	bar.View, bar.Content, bar.Offset = view, float32(s.Doc.Len())+view, float32(s.frame.top)
	return bar, false
}

func (s *State) statusLine(c *ui.Context) {
	var parts []string
	if done, total, err := s.Doc.Indexing(); err != nil {
		parts = append(parts, err.Error())
	} else if done < total {
		parts = append(parts, fmt.Sprintf("Indexing lines %d%%", done*percent/total))
	}
	if s.find.status != "" {
		parts = append(parts, s.find.status)
	}
	if s.notice != "" {
		parts = append(parts, s.notice)
	}
	if len(parts) == 0 {
		return
	}
	p := token.Of(c)
	ui.Row(c).Gap(token.GapSection.Px()).Height(token.StatusBarHeight.Px()).Padding(0, token.PadBarText.Px()).AlignItems(ui.Center).Shrink(0).FillWidth().
		Background(p.Layer.Panel).BorderWidth(token.BorderWidth.Px(), 0, 0, 0).BorderColor(p.Border.Default).Role(ui.RoleStatus).Children(func() {
		for _, part := range parts {
			design.Text(c, part, token.TextLabel).SingleLine()
		}
	})
}
