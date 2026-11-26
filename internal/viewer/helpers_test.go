package viewer

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/ZerGo0/editor/internal/host"
	"github.com/egoist/mygo/ui"
)

const (
	testWidth     = 900
	testHeight    = 600
	settleTimeout = 5 * time.Second
	settlePoll    = 20 * time.Millisecond
	commandWait   = 2 * time.Second
)

type openedAt struct {
	path      string
	line, col int
}

type fakeHost struct {
	host.Host
	updates chan func()
	mu      sync.Mutex
	notes   []string
	opened  []openedAt
}

func hasControl(tt *ui.Tester, label string) bool {
	_, ok := tt.Access(label)
	return ok
}

func newFakeHost() *fakeHost { return &fakeHost{updates: make(chan func(), 256)} }

func (f *fakeHost) Update(fn func()) { f.updates <- fn }

func (f *fakeHost) Notify(text string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.notes = append(f.notes, text)
}

func (f *fakeHost) OpenAt(path string, line, col int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.opened = append(f.opened, openedAt{path, line, col})
}

func (f *fakeHost) lastNote() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.notes) == 0 {
		return ""
	}
	return f.notes[len(f.notes)-1]
}

func (f *fakeHost) drain() {
	for {
		select {
		case fn := <-f.updates:
			fn()
		default:
			return
		}
	}
}

func settle(t *testing.T, h *fakeHost, tt *ui.Tester, done func() bool) {
	t.Helper()
	deadline := time.Now().Add(settleTimeout)
	for {
		h.drain()
		tt.Frame()
		if done() {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("condition was not reached in time")
		}
		select {
		case fn := <-h.updates:
			fn()
		case <-time.After(settlePoll):
		}
	}
}

func viewTester(s *State) *ui.Tester {
	return ui.NewTester(func(c *ui.Context) { s.View(c) }, testWidth, testHeight)
}

func recordCommands(s *State) chan []string {
	calls := make(chan []string, 16)
	s.run = func(name string, args ...string) error {
		calls <- append([]string{name}, args...)
		return nil
	}
	return calls
}

func nextCommand(t *testing.T, calls chan []string) []string {
	t.Helper()
	select {
	case call := <-calls:
		return call
	case <-time.After(commandWait):
		t.Fatal("no command was run")
		return nil
	}
}

func writePNG(t *testing.T, dir, name string, w, h int) string {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.NRGBA{R: uint8(x), G: uint8(y), B: 200, A: 128})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return writeFile(t, dir, name, buf.Bytes())
}

func writePNGImage(w, h int) *image.NRGBA {
	return image.NewNRGBA(image.Rect(0, 0, w, h))
}

func writeGIF(t *testing.T, dir, name string, frames int) string {
	t.Helper()
	palette := color.Palette{color.Black, color.White}
	anim := &gif.GIF{}
	for i := range frames {
		img := image.NewPaletted(image.Rect(0, 0, 4, 4), palette)
		img.SetColorIndex(i%4, i%4, 1)
		anim.Image = append(anim.Image, img)
		anim.Delay = append(anim.Delay, 10)
	}
	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, anim); err != nil {
		t.Fatal(err)
	}
	return writeFile(t, dir, name, buf.Bytes())
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func removeFile(path string) error { return os.Remove(path) }

func writeFile(t *testing.T, dir, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
