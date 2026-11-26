package viewer

import (
	"errors"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

type fakePlayer struct {
	playing bool
	pos     time.Duration
	dur     time.Duration
	ready   bool
	err     error
	volume  float64
	loop    bool
	closed  bool
	seeks   []time.Duration
}

func (f *fakePlayer) Play()                         { f.playing = true }
func (f *fakePlayer) Pause()                        { f.playing = false }
func (f *fakePlayer) Playing() bool                 { return f.playing }
func (f *fakePlayer) Position() time.Duration       { return f.pos }
func (f *fakePlayer) Duration() time.Duration       { return f.dur }
func (f *fakePlayer) Seek(d time.Duration)          { f.pos = d; f.seeks = append(f.seeks, d) }
func (f *fakePlayer) Ready() bool                   { return f.ready }
func (f *fakePlayer) Err() error                    { return f.err }
func (f *fakePlayer) SetVolume(v float64)           { f.volume = v }
func (f *fakePlayer) SetLoop(loop bool)             { f.loop = loop }
func (f *fakePlayer) Close()                        { f.closed = true }
func (f *fakePlayer) show(c *ui.Context) ui.Element { return ui.Box(c) }

type mediaFixture struct {
	s     *State
	p     *mediaPane
	h     *fakeHost
	tt    *ui.Tester
	opens int
	fake  *fakePlayer
}

func openMedia(t *testing.T, name string, kind Kind) *mediaFixture {
	t.Helper()
	fx := &mediaFixture{h: newFakeHost(), fake: &fakePlayer{dur: 90 * time.Second, ready: true, volume: 1}}
	path := writeFile(t, t.TempDir(), name, []byte("media bytes"))
	fx.s = newState(fx.h, path, kind, Options{})
	fx.p = &mediaPane{
		s: fx.s, audio: kind == Audio, volume: 1, checkEvery: time.Hour,
		open: func(string) (mediaPlayer, error) {
			fx.opens++
			return fx.fake, nil
		},
	}
	fx.p.start()
	fx.s.pane = fx.p
	t.Cleanup(fx.s.Close)
	fx.tt = viewTester(fx.s)
	return fx
}

func TestMediaPlayPauseAndTime(t *testing.T) {
	fx := openMedia(t, "clip.mp4", Video)
	if !fx.tt.HasText("0:00 / 1:30") || !hasControl(fx.tt, "Play") {
		t.Fatalf("texts: %q", fx.tt.Texts())
	}
	if err := fx.tt.Click("Play"); err != nil {
		t.Fatal(err)
	}
	if !fx.fake.playing || !hasControl(fx.tt, "Pause") {
		t.Fatalf("after Play: playing %v texts %q", fx.fake.playing, fx.tt.Texts())
	}
	fx.p.stopWatch()
	if err := fx.tt.Click("Pause"); err != nil {
		t.Fatal(err)
	}
	if fx.fake.playing {
		t.Error("Pause did not pause")
	}
}

func TestMediaSpaceTogglesPlayback(t *testing.T) {
	fx := openMedia(t, "clip.mp4", Video)
	fx.s.Handle().Focus()
	fx.tt.Frame()
	fx.tt.Key(0, ui.KeySpace)
	if !fx.fake.playing {
		t.Fatal("Space did not play")
	}
	fx.tt.Key(0, ui.KeySpace)
	if fx.fake.playing {
		t.Fatal("Space did not pause")
	}
}

func TestMediaVolumeLoopAndSeekControlsExist(t *testing.T) {
	fx := openMedia(t, "clip.mp4", Video)
	for _, label := range []string{"Seek", "Volume", "Loop"} {
		if _, ok := fx.tt.Access(label); !ok {
			t.Errorf("no %q control", label)
		}
	}
	if err := fx.tt.Click("Loop"); err != nil {
		t.Fatal(err)
	}
	if !fx.fake.loop {
		t.Error("the Loop toggle did not loop a video")
	}
}

func TestMediaShowsLoadingUntilDurationIsKnown(t *testing.T) {
	fx := openMedia(t, "song.mp3", Audio)
	fx.fake.ready, fx.fake.dur = false, 0
	fx.tt.Frame()
	if a, ok := fx.tt.Access("Play"); !ok || !a.Disabled {
		t.Errorf("Play while loading: %+v %v", a, ok)
	}
	if _, ok := fx.tt.Access(spinnerOverlayLabel); !ok {
		t.Error("no spinner while loading")
	}
}

func TestAudioLoopsAndStopsAtTheEnd(t *testing.T) {
	fx := openMedia(t, "song.mp3", Audio)
	fx.fake.playing, fx.fake.pos = true, fx.fake.dur
	fx.tt.Frame()
	if fx.fake.playing || fx.fake.pos != 0 {
		t.Errorf("audio at the end: playing %v pos %v", fx.fake.playing, fx.fake.pos)
	}
	fx.p.loop = true
	fx.fake.playing, fx.fake.pos = true, fx.fake.dur
	fx.tt.Frame()
	if !fx.fake.playing || fx.fake.pos != 0 {
		t.Errorf("looping audio at the end: playing %v pos %v", fx.fake.playing, fx.fake.pos)
	}
	fx.fake.pos = 5 * time.Second
	fx.tt.Advance(audioTick)
	if !fx.tt.HasText("0:05 / 1:30") {
		t.Errorf("the clock did not move without input: %q", fx.tt.Texts())
	}
}

func TestMediaIdleAsksForNoFrames(t *testing.T) {
	fx := openMedia(t, "song.mp3", Audio)
	if fx.tt.FramePending() {
		t.Error("paused audio keeps asking for frames")
	}
}

func TestMediaErrorOffersRetry(t *testing.T) {
	fx := openMedia(t, "clip.mkv", Video)
	fx.fake.err = errors.New("unsupported codec")
	fx.tt.Frame()
	if !fx.tt.HasText("Cannot play this file") || !fx.tt.HasText("unsupported codec") {
		t.Fatalf("texts: %q", fx.tt.Texts())
	}
	fx.p.open = func(string) (mediaPlayer, error) {
		fx.opens++
		fx.fake.err = nil
		return fx.fake, nil
	}
	if err := fx.tt.Click("Retry"); err != nil {
		t.Fatal(err)
	}
	if fx.opens != 2 || !hasControl(fx.tt, "Play") {
		t.Errorf("opens %d texts %q", fx.opens, fx.tt.Texts())
	}
}

func TestMediaOpenFailureIsShown(t *testing.T) {
	fx := openMedia(t, "clip.mp4", Video)
	fx.p.open = func(string) (mediaPlayer, error) { return nil, errors.New("no media framework") }
	fx.s.Reload()
	fx.tt.Frame()
	if !fx.tt.HasText("no media framework") || !fx.fake.closed {
		t.Errorf("texts %q closed %v", fx.tt.Texts(), fx.fake.closed)
	}
}

func TestMediaPausesWhenTheViewIsNoLongerBuilt(t *testing.T) {
	fx := openMedia(t, "song.mp3", Audio)
	fx.p.checkEvery = 5 * time.Millisecond
	fx.p.toggle()
	if !fx.fake.playing {
		t.Fatal("toggle did not play")
	}
	deadline := time.After(settleTimeout)
	for fx.fake.playing {
		select {
		case fn := <-fx.h.updates:
			fn()
		case <-deadline:
			t.Fatal("the watchdog never paused the player")
		}
	}
	if fx.fake.playing || fx.p.stop != nil {
		t.Errorf("playing %v watchdog %v", fx.fake.playing, fx.p.stop != nil)
	}
}

func TestMediaKeepsPlayingWhileViewsBuild(t *testing.T) {
	fx := openMedia(t, "song.mp3", Audio)
	fx.p.checkEvery = 20 * time.Millisecond
	fx.p.toggle()
	deadline := time.Now().Add(150 * time.Millisecond)
	for time.Now().Before(deadline) {
		fx.tt.Frame()
		fx.h.drain()
		time.Sleep(5 * time.Millisecond)
	}
	if !fx.fake.playing {
		t.Error("the watchdog paused a player whose view is built")
	}
	fx.p.stopWatch()
}

func TestMediaPauseAndClose(t *testing.T) {
	fx := openMedia(t, "song.mp3", Audio)
	fx.p.toggle()
	fx.s.Pause()
	if fx.fake.playing || fx.p.stop != nil {
		t.Error("Pause left the player running")
	}
	fx.s.Close()
	if !fx.fake.closed {
		t.Error("Close did not close the player")
	}
}

func TestMediaInfoLine(t *testing.T) {
	fx := openMedia(t, "clip.mp4", Video)
	if got := fx.p.info(); got != "1:30  \u00b7  11 bytes" {
		t.Errorf("info = %q", got)
	}
}
