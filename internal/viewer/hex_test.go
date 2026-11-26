package viewer

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ZerGo0/editor/internal/viewer/hexdump"
	"github.com/egoist/mygo/ui"
)

func binaryData(size int) []byte {
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(i)
	}
	return data
}

func openHex(t *testing.T, data []byte, opts Options) (*State, *hexPane, *fakeHost, *ui.Tester) {
	t.Helper()
	h := newFakeHost()
	path := writeFile(t, t.TempDir(), "blob.bin", data)
	s, err := Open(h, path, Hex, opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	p := s.pane.(*hexPane)
	tt := viewTester(s)
	settle(t, h, tt, func() bool { return p.src != nil })
	return s, p, h, tt
}

func TestHexBannerAndOpenAsText(t *testing.T) {
	opened := 0
	_, p, _, tt := openHex(t, binaryData(1234), Options{OpenAsText: func() { opened++ }})
	if !tt.HasText("Binary file, 1,234 bytes.") {
		t.Fatalf("texts: %q", tt.Texts())
	}
	if err := tt.Click("Open as Text Anyway"); err != nil {
		t.Fatal(err)
	}
	if opened != 1 {
		t.Errorf("callback ran %d times", opened)
	}
	if p.size != 1234 {
		t.Errorf("size = %d", p.size)
	}
}

func TestHexHidesOpenAsTextWithoutCallback(t *testing.T) {
	_, _, _, tt := openHex(t, binaryData(10), Options{})
	if tt.HasText("Open as Text Anyway") {
		t.Error("the button shows without a callback")
	}
}

func TestHexEmptyFile(t *testing.T) {
	_, _, _, tt := openHex(t, nil, Options{})
	if !tt.HasText("This file is empty") || !tt.HasText("Binary file, 0 bytes.") {
		t.Errorf("texts: %q", tt.Texts())
	}
}

func TestHexDoesNotAskForFramesWhenIdle(t *testing.T) {
	_, p, h, tt := openHex(t, binaryData(hexdump.PageSize*2), Options{})
	settle(t, h, tt, func() bool {
		_, err := p.src.Page(0)
		return err == nil
	})
	if tt.FramePending() {
		t.Error("an idle hex view keeps asking for frames")
	}
}

func TestHexScrollingAndKeys(t *testing.T) {
	size := hexdump.PageSize * 3
	s, p, _, tt := openHex(t, binaryData(size), Options{})
	box, ok := tt.Find("Hex dump")
	if !ok {
		t.Fatal("no dump")
	}
	tt.Scroll(box.X+10, box.Y+10, 0, p.lineH*5)
	if p.top != 5 {
		t.Errorf("top after scrolling five lines = %d", p.top)
	}
	s.Handle().Focus()
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyDown)
	if p.caret != int64(size-1) || p.top != p.maxTop() || p.top == 0 {
		t.Errorf("Cmd+Down: caret %d top %d max %d", p.caret, p.top, p.maxTop())
	}
	tt.Key(ui.Cmd, ui.KeyUp)
	if p.caret != 0 || p.top != 0 {
		t.Errorf("Cmd+Up: caret %d top %d", p.caret, p.top)
	}
	tt.Key(0, ui.KeyRight)
	tt.Key(0, ui.KeyDown)
	if p.caret != 1+hexdump.BytesPerRow {
		t.Errorf("caret after Right and Down = %d", p.caret)
	}
	tt.Key(ui.Shift, ui.KeyRight)
	if a, b, _ := p.selected(); b-a != 2 {
		t.Errorf("Shift+Right selected %d bytes", b-a)
	}
	tt.Key(0, ui.KeyEscape)
	if p.hasSel {
		t.Error("Escape kept the selection")
	}
	tt.Key(0, ui.KeyPageDown)
	if p.top != 0 && p.caret < int64(p.fullRows()) {
		t.Errorf("Page Down left caret %d top %d", p.caret, p.top)
	}
}

func TestHexGoToOffset(t *testing.T) {
	_, p, _, tt := openHex(t, binaryData(hexdump.PageSize*2), Options{})
	if err := tt.Click("Go to offset"); err != nil {
		t.Fatal(err)
	}
	tt.Type("0x10020")
	tt.Key(0, ui.KeyEnter)
	if p.caret != 0x10020 || !p.hasSel {
		t.Fatalf("caret = %#x selected = %v", p.caret, p.hasSel)
	}
	if want := hexdump.RowOf(0x10020); p.top > want || p.top+int64(p.fullRows()) <= want {
		t.Errorf("row %d is outside the view starting at %d", want, p.top)
	}
	if !tt.HasText("Offset 0x10020 (65,568)") {
		t.Errorf("texts: %q", tt.Texts())
	}
}

func TestHexGoToOffsetRejectsBadInput(t *testing.T) {
	_, p, _, tt := openHex(t, binaryData(64), Options{})
	if err := tt.Click("Go to offset"); err != nil {
		t.Fatal(err)
	}
	tt.Type("9999")
	tt.Key(0, ui.KeyEnter)
	if p.goErr == "" || p.hasSel {
		t.Fatalf("goErr %q selected %v", p.goErr, p.hasSel)
	}
	if !tt.HasText(p.goErr) {
		t.Errorf("the error is not shown: %q", tt.Texts())
	}
	tt.Type("1")
	if p.goErr != "" {
		t.Error("editing the field kept the error")
	}
}

func TestHexSelectByMouseAndCopy(t *testing.T) {
	data := binaryData(256)
	_, p, h, tt := openHex(t, data, Options{})
	box, _ := tt.Find("Hex dump")
	pointAt := func(row int, index int) (float32, float32) {
		x := box.X + hexPadX + (float32(p.layout.HexStart()+hexdump.HexColumn(index))+0.5)*p.charW
		return x, box.Y + (float32(row)+0.5)*p.lineH
	}
	x1, y1 := pointAt(1, 2)
	x2, y2 := pointAt(1, 5)
	tt.Press(x1, y1)
	tt.Move(x2, y2)
	tt.Release(x2, y2)
	if a, b, ok := p.selected(); !ok || a != 18 || b != 22 {
		t.Fatalf("selection = %d, %d, %v; want 18, 22", a, b, ok)
	}
	tt.Key(ui.Cmd, ui.KeyC)
	settle(t, h, tt, func() bool { return tt.Clipboard() != "" })
	if want := hexdump.HexString(data[18:22]); tt.Clipboard() != want {
		t.Errorf("clipboard %q, want %q", tt.Clipboard(), want)
	}
	if !strings.Contains(h.lastNote(), "Copied 4 bytes as hex") {
		t.Errorf("note %q", h.lastNote())
	}
	tt.SetClipboard("")
	if err := tt.Click("Copy Text"); err != nil {
		t.Fatal(err)
	}
	settle(t, h, tt, func() bool { return tt.Clipboard() != "" })
	if want := hexdump.FormatASCII(data[18:22]); tt.Clipboard() != want {
		t.Errorf("text clipboard %q, want %q", tt.Clipboard(), want)
	}
}

func TestHexCopyButtonsNeedASelection(t *testing.T) {
	_, p, _, tt := openHex(t, binaryData(64), Options{})
	if a, ok := tt.Access("Copy Hex"); !ok || !a.Disabled {
		t.Errorf("Copy Hex without a selection: %+v %v", a, ok)
	}
	p.selectAll()
	tt.Frame()
	if a, _ := tt.Access("Copy Hex"); a.Disabled {
		t.Error("Copy Hex stays disabled with a selection")
	}
}

func TestHexCopyRefusesHugeSelections(t *testing.T) {
	s, p, h, tt := openHex(t, binaryData(64), Options{})
	p.size = hexCopyLimit + 1
	p.anchor, p.caret, p.hasSel = 0, hexCopyLimit, true
	p.copySelection(true)
	if !strings.Contains(h.lastNote(), "was not copied") {
		t.Errorf("note %q", h.lastNote())
	}
	_ = s
	_ = tt
}

func TestHexReloadShowsNewSize(t *testing.T) {
	s, p, h, tt := openHex(t, binaryData(100), Options{})
	writeFile(t, tDir(s), "blob.bin", binaryData(200))
	s.Reload()
	settle(t, h, tt, func() bool { return p.size == 200 })
	if !tt.HasText("Binary file, 200 bytes.") {
		t.Errorf("texts: %q", tt.Texts())
	}
}

func tDir(s *State) string { return strings.TrimSuffix(s.path, "/blob.bin") }

func TestHexMissingFileShowsRetry(t *testing.T) {
	s, p, h, tt := openHex(t, binaryData(10), Options{})
	if err := removeFile(s.path); err != nil {
		t.Fatal(err)
	}
	s.Reload()
	settle(t, h, tt, func() bool { return p.err != nil })
	if !tt.HasText("Cannot read this file") {
		t.Fatalf("texts: %q", tt.Texts())
	}
	writeFile(t, tDir(s), "blob.bin", bytes.Repeat([]byte{0}, 5))
	if err := tt.Click("Retry"); err != nil {
		t.Fatal(err)
	}
	settle(t, h, tt, func() bool { return p.err == nil && p.size == 5 })
}

func TestHexUsesTheFontOptions(t *testing.T) {
	_, p, _, _ := openHex(t, binaryData(16), Options{FontFamily: "Menlo", FontSize: 15})
	if f := p.font(); f.Family != "Menlo" || f.Size != 15 {
		t.Errorf("font = %+v", f)
	}
	_, def, _, _ := openHex(t, binaryData(16), Options{})
	if f := def.font(); f.Family != defaultMonoFam || f.Size != defaultMonoPt {
		t.Errorf("default font = %+v", f)
	}
}
