package viewer

import (
	"errors"
	"fmt"
	"math"
	"os"

	"github.com/ZerGo0/editor/internal/viewer/hexdump"
	"github.com/egoist/mygo/ui"
)

const (
	hexPadX          = 12
	hexLineFactor    = 1.5
	hexShapeCacheMax = 1024
	hexMeasureSample = "0"
	hexCopyLimit     = 16 << 20
	hexGotoWidth     = 180
	hexBannerGap     = 8
	hexBannerPadY    = 4
	hexBannerPadX    = 12
	hexDumpLabel     = "Hex dump"
	hexUnreadable    = "unreadable"
	hexNoSelection   = "No selection"
)

type shapedText struct {
	layout *ui.TextLayout
	w, h   float32
}

type barDrag struct {
	active bool
	grab   float32
}

type hexPane struct {
	s        *State
	file     *os.File
	src      *hexdump.Source
	size     int64
	layout   hexdump.Layout
	loading  bool
	err      error
	gen      uint64
	top      int64
	anchor   int64
	caret    int64
	hasSel   bool
	goTo     string
	goErr    string
	goField  ui.Handle
	viewW    float32
	viewH    float32
	lineH    float32
	charW    float32
	wheelY   float32
	dragging bool
	bar      barDrag
	barStyle ui.ScrollbarStyle
	shapes   map[string]*shapedText
}

func newHexPane(s *State) *hexPane {
	p := &hexPane{s: s, shapes: map[string]*shapedText{}}
	p.load()
	return p
}

type openedFile struct {
	file *os.File
	size int64
}

func (p *hexPane) load() {
	p.gen++
	gen := p.gen
	path := p.s.path
	p.loading, p.err = true, nil
	go func() {
		f, err := os.Open(path)
		var info os.FileInfo
		if err == nil {
			info, err = f.Stat()
		}
		var opened openedFile
		if err == nil {
			opened = openedFile{file: f, size: info.Size()}
		} else if f != nil {
			f.Close()
		}
		p.s.h.Update(func() {
			if gen != p.gen || p.s.closed.Load() {
				if opened.file != nil {
					opened.file.Close()
				}
				return
			}
			p.finish(opened, err)
		})
	}()
	p.s.redraw()
}

func (p *hexPane) finish(opened openedFile, err error) {
	p.loading = false
	if err != nil {
		p.err = fmt.Errorf("open file: %w", err)
		return
	}
	p.releaseFile()
	p.file, p.size = opened.file, opened.size
	p.src = hexdump.NewSource(opened.file, opened.size, p.s.redraw)
	p.layout = hexdump.Layout{Digits: hexdump.OffsetDigits(opened.size)}
	p.top = min(p.top, max(hexdump.Rows(opened.size)-1, 0))
	if p.hasSel && (p.anchor >= opened.size || p.caret >= opened.size) {
		p.hasSel = false
	}
}

func (p *hexPane) releaseFile() {
	if p.file != nil {
		p.file.Close()
		p.file, p.src = nil, nil
	}
}

func (p *hexPane) reload() { p.load() }

func (p *hexPane) pause() {}

func (p *hexPane) close() {
	p.gen++
	p.releaseFile()
}

func (p *hexPane) info() string {
	a, b, ok := p.selected()
	if !ok {
		return hexNoSelection
	}
	return joinInfo(fmt.Sprintf("Offset 0x%x (%s)", a, formatCount(a)), fmt.Sprintf("%s bytes selected", formatCount(b-a)))
}

func (p *hexPane) selected() (start, end int64, ok bool) {
	if !p.hasSel {
		return 0, 0, false
	}
	return hexdump.Range(p.anchor, p.caret, p.size)
}

func (p *hexPane) controls(c *ui.Context) {
	if p.src == nil {
		return
	}
	_, _, ok := p.selected()
	if ui.Button(c.Key("copy-hex"), "Copy Hex").FontSize(smallFontSize).Tooltip("Copy the selected bytes as hex (Cmd+C)").Disabled(!ok).Clicked() {
		p.copySelection(true)
	}
	if ui.Button(c.Key("copy-text"), "Copy Text").FontSize(smallFontSize).Tooltip("Copy the selected bytes as text (Cmd+Shift+C)").Disabled(!ok).Clicked() {
		p.copySelection(false)
	}
}

func (p *hexPane) body(c *ui.Context) {
	switch {
	case p.err != nil:
		errorView(c, "Cannot read this file", p.err, p.load)
	case p.src == nil:
		loadingView(c, "Reading file")
	default:
		p.banner(c)
		if p.size == 0 {
			message(c, "This file is empty", "There are no bytes to show.", nil)
			return
		}
		p.dump(c)
	}
}

func (p *hexPane) banner(c *ui.Context) {
	t := c.Theme()
	ui.Row(c.Key("banner")).Gap(hexBannerGap).AlignItems(ui.Center).Padding(hexBannerPadY, hexBannerPadX).Shrink(0).FillWidth().
		Background(t.Surface).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
		ui.Text(c, fmt.Sprintf("Binary file, %s bytes.", formatCount(p.size))).FontSize(smallFontSize)
		if p.s.opts.OpenAsText != nil {
			if ui.Button(c.Key("open-as-text"), "Open as text anyway").FontSize(smallFontSize).Clicked() {
				p.s.opts.OpenAsText()
			}
		}
		ui.Spacer(c)
		if p.goErr != "" {
			ui.Text(c, p.goErr).FontSize(smallFontSize).TextColor(t.Danger).SingleLine()
		}
		field := ui.TextInput(c.Key("goto"), &p.goTo).Placeholder("Go to offset (Cmd+G)").Label("Go to offset").
			FontSize(smallFontSize).Width(hexGotoWidth).Disabled(p.size == 0).Bind(&p.goField)
		if field.Changed() {
			p.goErr = ""
		}
		if field.Submitted() {
			p.jump()
		}
	})
}

func (p *hexPane) jump() {
	off, err := hexdump.ParseOffset(p.goTo, p.size)
	if err != nil {
		p.goErr = err.Error()
		return
	}
	p.goErr = ""
	p.anchor, p.caret, p.hasSel = off, off, true
	p.top = max(hexdump.RowOf(off)-int64(p.fullRows()/2), 0)
	p.clampTop()
	p.s.focus.Focus()
}

func (p *hexPane) dump(c *ui.Context) {
	t := c.Theme()
	p.barStyle = ui.ScrollbarStyle{}.Resolve(t)
	p.measure()
	e := ui.Box(c.Key("dump")).Grow(1).FillWidth().Focusable().FocusRing(false).Clip().Cursor(ui.CursorText).
		Role(ui.RoleGroup).Label(hexDumpLabel).Bind(&p.s.focus)
	e.HandleInput(p.input).Draw(func(pnt *ui.Painter, r ui.Rect) { p.paint(pnt, r, t) })
}

func (p *hexPane) font() ui.Font {
	return ui.Font{Family: p.s.opts.FontFamily, Size: p.s.opts.FontSize}
}

func (p *hexPane) measure() {
	if p.lineH != 0 {
		return
	}
	p.lineH = float32(math.Round(float64(p.s.opts.FontSize) * hexLineFactor))
	p.charW = p.shape(hexMeasureSample).w
}

func (p *hexPane) shape(text string) *shapedText {
	if sh, ok := p.shapes[text]; ok {
		return sh
	}
	if len(p.shapes) >= hexShapeCacheMax {
		clear(p.shapes)
	}
	l := ui.ShapeText(text, p.font(), 0)
	w, h := l.Size()
	sh := &shapedText{layout: l, w: w, h: h}
	p.shapes[text] = sh
	return sh
}

func (p *hexPane) fullRows() int { return max(1, int(p.viewH/p.lineH)) }

func (p *hexPane) visibleRows() int { return int(math.Ceil(float64(p.viewH/p.lineH))) + 1 }

func (p *hexPane) paint(pnt *ui.Painter, r ui.Rect, t *ui.Theme) {
	if r.W != p.viewW || r.H != p.viewH {
		p.viewW, p.viewH = r.W, r.H
		p.s.svc.Invalidate()
	}
	pnt.Fill(r, t.Background, 0)
	rows := hexdump.Rows(p.size)
	x0 := r.X + hexPadX
	hexX := x0 + float32(p.layout.HexStart())*p.charW
	asciiX := x0 + float32(p.layout.ASCIIStart())*p.charW
	selA, selB, hasSel := p.selected()
	pageIdx, page, pageErr := int64(-1), []byte(nil), error(nil)
	for i := range p.visibleRows() {
		row := p.top + int64(i)
		if row >= rows {
			break
		}
		y := r.Y + float32(i)*p.lineH
		start, end := hexdump.RowBounds(row, p.size)
		if idx := hexdump.PageOf(start); idx != pageIdx {
			page, pageErr = p.src.Page(idx)
			pageIdx = idx
		}
		if hasSel {
			p.paintSelection(pnt, t, start, end, selA, selB, hexX, asciiX, y)
		}
		p.drawText(pnt, hexdump.FormatOffset(start, p.layout.Digits), x0, y, t.TextMuted)
		switch {
		case pageErr == nil:
			base, _ := hexdump.PageBounds(pageIdx, p.size)
			data := page[start-base : end-base]
			p.drawText(pnt, hexdump.FormatHex(data), hexX, y, t.Text)
			p.drawText(pnt, hexdump.FormatASCII(data), asciiX, y, t.Text)
		case !errors.Is(pageErr, hexdump.ErrLoading):
			p.drawText(pnt, hexUnreadable, hexX, y, t.Danger)
		}
	}
	bar := p.scrollbar()
	bar.Box.X, bar.Box.Y = r.X, r.Y
	if bar.Scrolls() {
		pnt.Scrollbar(bar, ui.ScrollbarState{Dragging: p.bar.active})
	}
}

func (p *hexPane) drawText(pnt *ui.Painter, text string, x, y float32, color ui.Color) {
	sh := p.shape(text)
	pnt.TextLayout(sh.layout, x, y+(p.lineH-sh.h)/2, color)
}

func (p *hexPane) paintSelection(pnt *ui.Painter, t *ui.Theme, start, end, selA, selB int64, hexX, asciiX, y float32) {
	from, to := max(selA, start), min(selB, end)
	if from >= to {
		return
	}
	first, last := int(from-start), int(to-start-1)
	hexStart := hexX + float32(hexdump.HexColumn(first))*p.charW
	hexEnd := hexX + float32(hexdump.HexColumn(last)+2)*p.charW
	pnt.Fill(ui.Rect{X: hexStart, Y: y, W: hexEnd - hexStart, H: p.lineH}, t.Selection, 0)
	pnt.Fill(ui.Rect{X: asciiX + float32(first)*p.charW, Y: y, W: float32(last-first+1) * p.charW, H: p.lineH}, t.Selection, 0)
}

func (p *hexPane) scrollbar() ui.Scrollbar {
	return ui.Scrollbar{
		Box: ui.Rect{W: p.viewW, H: p.viewH}, Style: p.barStyle,
		View: float32(p.fullRows()), Content: float32(hexdump.Rows(p.size)), Offset: float32(p.top),
	}
}

func (p *hexPane) maxTop() int64 { return max(hexdump.Rows(p.size)-int64(p.fullRows()), 0) }

func (p *hexPane) clampTop() { p.top = min(max(p.top, 0), p.maxTop()) }

func (p *hexPane) scrollRows(n int64) {
	p.top += n
	p.clampTop()
}

func (p *hexPane) input(ev ui.InputEvent) bool {
	switch ev.Kind {
	case ui.InputKeyDown:
		return p.key(ev.Mods, ev.Key)
	case ui.InputCommand:
		return p.command(ev.Text)
	case ui.InputPointerDown:
		return p.pointerDown(ev)
	case ui.InputPointerMove:
		return p.pointerMove(ev)
	case ui.InputPointerUp:
		held := p.dragging || p.bar.active
		p.dragging, p.bar.active = false, false
		return held
	case ui.InputScroll:
		p.wheelY += ev.DY
		if n := int64(p.wheelY / p.lineH); n != 0 {
			p.scrollRows(n)
			p.wheelY -= float32(n) * p.lineH
		}
		return true
	}
	return false
}

func (p *hexPane) command(name string) bool {
	switch name {
	case "copy":
		p.copySelection(true)
	case "selectAll":
		p.selectAll()
	default:
		return false
	}
	return true
}

func (p *hexPane) selectAll() {
	p.anchor, p.caret, p.hasSel = 0, p.size-1, p.size > 0
}

func (p *hexPane) key(mods ui.Modifiers, key ui.Key) bool {
	extend := mods&ui.Shift != 0
	switch mods &^ ui.Shift {
	case 0:
		return p.plainKey(key, extend)
	case ui.Cmd:
		switch key {
		case ui.KeyA:
			p.selectAll()
		case ui.KeyC:
			p.copySelection(!extend)
		case ui.KeyG:
			p.goField.SelectAll()
		case ui.KeyUp, ui.KeyHome:
			p.moveTo(0, extend)
		case ui.KeyDown, ui.KeyEnd:
			p.moveTo(p.size-1, extend)
		default:
			return false
		}
		return true
	}
	return false
}

func (p *hexPane) plainKey(key ui.Key, extend bool) bool {
	page := int64(max(p.fullRows()-1, 1)) * hexdump.BytesPerRow
	switch key {
	case ui.KeyLeft:
		p.moveBy(-1, extend)
	case ui.KeyRight:
		p.moveBy(1, extend)
	case ui.KeyUp:
		p.moveBy(-hexdump.BytesPerRow, extend)
	case ui.KeyDown:
		p.moveBy(hexdump.BytesPerRow, extend)
	case ui.KeyPageUp:
		p.moveBy(-page, extend)
	case ui.KeyPageDown:
		p.moveBy(page, extend)
	case ui.KeyHome:
		p.moveTo(p.caret/hexdump.BytesPerRow*hexdump.BytesPerRow, extend)
	case ui.KeyEnd:
		p.moveTo(p.caret/hexdump.BytesPerRow*hexdump.BytesPerRow+hexdump.BytesPerRow-1, extend)
	case ui.KeyEscape:
		held := p.hasSel
		p.hasSel = false
		return held
	default:
		return false
	}
	return true
}

func (p *hexPane) moveBy(delta int64, extend bool) {
	if !p.hasSel {
		p.moveTo(p.top*hexdump.BytesPerRow, false)
		return
	}
	p.moveTo(p.caret+delta, extend)
}

func (p *hexPane) moveTo(off int64, extend bool) {
	if p.size == 0 {
		return
	}
	off = min(max(off, 0), p.size-1)
	p.caret = off
	if !extend || !p.hasSel {
		p.anchor = off
	}
	p.hasSel = true
	p.reveal(off)
}

func (p *hexPane) reveal(off int64) {
	row := hexdump.RowOf(off)
	switch {
	case row < p.top:
		p.top = row
	case row >= p.top+int64(p.fullRows()):
		p.top = row - int64(p.fullRows()) + 1
	}
	p.clampTop()
}

func (p *hexPane) pointerDown(ev ui.InputEvent) bool {
	if ev.Button != pointerPrimary {
		return false
	}
	p.s.focus.Focus()
	if ev.X >= p.viewW-p.barStyle.TrackWidth {
		if bar := p.scrollbar(); bar.Scrolls() {
			start, length := bar.Thumb()
			p.bar = barDrag{active: true, grab: ev.Y - start}
			if ev.Y < start || ev.Y >= start+length {
				p.bar.grab = length / 2
				p.scrollToBar(bar.OffsetAt(ev.Y - p.bar.grab))
			}
			return true
		}
	}
	off, ok := p.byteAt(ev.X, ev.Y)
	if !ok {
		return true
	}
	p.caret = off
	if ev.Mods&ui.Shift == 0 || !p.hasSel {
		p.anchor = off
	}
	p.hasSel, p.dragging = true, true
	return true
}

func (p *hexPane) pointerMove(ev ui.InputEvent) bool {
	switch {
	case p.bar.active:
		p.scrollToBar(p.scrollbar().OffsetAt(ev.Y - p.bar.grab))
	case p.dragging && ev.Button == pointerPrimary:
		if off, ok := p.byteAt(ev.X, ev.Y); ok {
			p.caret = off
		}
	default:
		return false
	}
	return true
}

func (p *hexPane) scrollToBar(offset float32) {
	p.top = int64(offset)
	p.clampTop()
}

func (p *hexPane) byteAt(x, y float32) (int64, bool) {
	if p.charW == 0 || p.size == 0 {
		return 0, false
	}
	row := min(max(p.top+int64(math.Floor(float64(y/p.lineH))), 0), hexdump.Rows(p.size)-1)
	col := int(math.Floor(float64((x - hexPadX) / p.charW)))
	hit, ok := p.layout.Hit(col)
	if !ok {
		return 0, false
	}
	return min(row*hexdump.BytesPerRow+int64(hit.Index), p.size-1), true
}

func (p *hexPane) copySelection(asHex bool) {
	start, end, ok := p.selected()
	if !ok || p.file == nil {
		return
	}
	if end-start > hexCopyLimit {
		p.s.h.Notify(fmt.Sprintf("Selection is larger than %d MB and was not copied", hexCopyLimit>>20))
		return
	}
	file, gen := p.file, p.gen
	go func() {
		data, err := hexdump.ReadRange(file, start, end)
		p.s.publish(func() {
			if gen != p.gen {
				return
			}
			if err != nil {
				p.s.h.Notify(err.Error())
				return
			}
			text, kind := hexdump.HexString(data), "hex"
			if !asHex {
				text, kind = hexdump.FormatASCII(data), "text"
			}
			p.s.svc.WriteClipboard(text)
			p.s.h.Notify(fmt.Sprintf("Copied %s bytes as %s", formatCount(end-start), kind))
		})
	}()
}
