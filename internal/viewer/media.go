package viewer

import (
	"fmt"
	"os"
	"sync/atomic"
	"time"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/design"
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/detail"
	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/video"
)

const (
	audioTick           = 250 * time.Millisecond
	audioEndSlack       = 150 * time.Millisecond
	hiddenCheckInterval = time.Second
	spinnerOverlayLabel = "Loading media"
	mediaLabel          = "Media player"
	volumeMax           = 1
	minSeekSeconds      = 1
)

// TECHDEBT: Audio-only files ride AVFoundation, which never reports their end, so the pane polls the clock every audioTick; move to package audio once the go-mp3, oggvorbis and flac modules are in go.mod.
type mediaPlayer interface {
	Play()
	Pause()
	Playing() bool
	Position() time.Duration
	Duration() time.Duration
	Seek(time.Duration)
	Ready() bool
	Err() error
	SetVolume(float64)
	SetLoop(bool)
	Close()
	show(c *ui.Context) ui.Element
}

type systemPlayer struct{ *ui.VideoPlayer }

func (p systemPlayer) SetVolume(v float64) { p.VideoPlayer.SetVolume(v) }

func (p systemPlayer) SetLoop(loop bool) { p.VideoPlayer.SetLoop(loop) }

func (p systemPlayer) show(c *ui.Context) ui.Element {
	return ui.Video(c.Key("video"), p.VideoPlayer)
}

func openSystemPlayer(path string) (mediaPlayer, error) {
	vp, err := video.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open media: %w", err)
	}
	vp.Pause()
	return systemPlayer{vp}, nil
}

type mediaPane struct {
	s          *State
	audio      bool
	open       func(path string) (mediaPlayer, error)
	player     mediaPlayer
	err        error
	volume     float64
	loop       bool
	pos        float64
	size       int64
	seq        atomic.Uint64
	stop       chan struct{}
	checkEvery time.Duration
}

func newMediaPane(s *State) *mediaPane {
	p := &mediaPane{s: s, audio: s.kind == Audio, open: openSystemPlayer, volume: 1, checkEvery: hiddenCheckInterval}
	p.start()
	return p
}

func (p *mediaPane) start() {
	p.stopWatch()
	p.closePlayer()
	p.err = nil
	info, err := os.Stat(p.s.path)
	if err != nil {
		p.err = err
		return
	}
	p.size = info.Size()
	player, err := p.open(p.s.path)
	if err != nil {
		p.err = err
		return
	}
	player.SetVolume(p.volume)
	player.SetLoop(p.loop && !p.audio)
	p.player = player
	p.s.redraw()
}

func (p *mediaPane) closePlayer() {
	if p.player != nil {
		p.player.Close()
		p.player = nil
	}
}

func (p *mediaPane) reload() { p.start() }

func (p *mediaPane) pause() {
	if p.player != nil && p.player.Playing() {
		p.player.Pause()
	}
	p.stopWatch()
}

func (p *mediaPane) close() {
	p.stopWatch()
	p.closePlayer()
}

func (p *mediaPane) startWatch() {
	if p.stop != nil {
		return
	}
	stop := make(chan struct{})
	p.stop = stop
	last := p.seq.Load()
	every := p.checkEvery
	applog.Go(func() {
		tick := time.NewTicker(every)
		defer tick.Stop()
		for {
			select {
			case <-stop:
				return
			case <-tick.C:
				cur := p.seq.Load()
				if cur == last {
					p.s.publish(func() {
						if p.stop == stop {
							p.pause()
						}
					})
					return
				}
				last = cur
			}
		}
	})
}

func (p *mediaPane) stopWatch() {
	if p.stop != nil {
		close(p.stop)
		p.stop = nil
	}
}

func (p *mediaPane) toggle() {
	if p.player == nil {
		return
	}
	if p.player.Playing() {
		p.player.Pause()
		p.stopWatch()
		return
	}
	p.player.Play()
	p.startWatch()
}

func (p *mediaPane) failure() error {
	if p.err != nil {
		return p.err
	}
	if p.player != nil {
		return p.player.Err()
	}
	return nil
}

func (p *mediaPane) info() string {
	if p.player == nil {
		return ""
	}
	dur := ""
	if d := p.player.Duration(); d > 0 {
		dur = formatClock(d)
	}
	return joinInfo(dur, formatSize(p.size))
}

func (p *mediaPane) controls(c *ui.Context) {}

func (p *mediaPane) body(c *ui.Context) {
	p.seq.Add(1)
	if err := p.failure(); err != nil {
		p.stopWatch()
		detail.Failed(c, "Cannot play this file", err, p.start)
		return
	}
	pl := p.player
	loaded := pl.Ready() || pl.Duration() > 0
	p.advance(c)
	col := ui.Column(c.Key("media")).Grow(1).FillWidth().Focusable().FocusRing(false).
		Role(ui.RoleGroup).Label(mediaLabel).Bind(&p.s.focus)
	if col.Shortcut(0, ui.KeySpace) {
		p.toggle()
	}
	col.Children(func() {
		p.stage(c, loaded)
		p.transport(c, loaded)
	})
}

func (p *mediaPane) advance(c *ui.Context) {
	pl := p.player
	if !p.audio || !pl.Playing() {
		return
	}
	if dur := pl.Duration(); dur > 0 && pl.Position() >= dur-audioEndSlack {
		pl.Seek(0)
		if !p.loop {
			pl.Pause()
			p.stopWatch()
			return
		}
	}
	c.After(audioTick)
}

func (p *mediaPane) stage(c *ui.Context, loaded bool) {
	pal := token.Of(c)
	ui.Column(c.Key("stage")).Grow(1).FillWidth().Center().Gap(token.GapRowIcon.Px()).Background(pal.Layer.Editor).Children(func() {
		surface := p.player.show(c)
		if p.audio {
			surface.Size(0, 0).AccessHidden(true).IgnoreInput(true)
			design.Text(c, p.s.name(), token.TextStrong)
			design.Text(c, p.s.kind.String(), token.TextLabel)
		} else {
			surface.Grow(1).FillWidth().Fit(ui.Contain)
		}
		if !loaded {
			feedback.Spinner(c, spinnerOverlayLabel).Absolute().Top(token.GapControl.Px()).Right(token.GapControl.Px())
		}
	})
}

func (p *mediaPane) transport(c *ui.Context, loaded bool) {
	pal := token.Of(c)
	pl := p.player
	dur := pl.Duration()
	ui.Row(c.Key("transport")).Gap(token.GapControl.Px()).AlignItems(ui.Center).Height(token.ToolbarHeight.Px()).Padding(0, token.PadRow.Px()).Shrink(0).FillWidth().
		Background(pal.Layer.Panel).BorderWidth(token.BorderWidth.Px(), 0, 0, 0).BorderColor(pal.Border.Default).Role(ui.RoleToolbar).Label("Media controls").Children(func() {
		label, icon := "Play", icons.Play
		if pl.Playing() {
			label, icon = "Pause", icons.Pause
		}
		if action.IconClicked(c, icon, label, action.IconOptions{ID: "play", Disabled: !loaded}) {
			p.toggle()
		}
		design.Text(c.Key("time"), formatClock(pl.Position())+" / "+formatClock(dur), token.TextLabel).Width(token.ClockWidth.Px()).SingleLine()
		p.pos = pl.Position().Seconds()
		seek := ui.Slider(c.Key("seek"), &p.pos, 0, max(dur.Seconds(), minSeekSeconds)).Label("Seek").Grow(1).Disabled(!loaded || dur <= 0)
		if seek.Changed() {
			pl.Seek(time.Duration(p.pos * float64(time.Second)))
		}
		ui.Icon(c, icons.Volume).Size(token.IconSize.Px(), token.IconSize.Px()).TextColor(pal.Icon).Shrink(0).AccessHidden(true)
		volume := ui.Slider(c.Key("volume"), &p.volume, 0, volumeMax).Label("Volume").Width(token.SliderWidth.Px())
		if volume.Changed() {
			pl.SetVolume(p.volume)
		}
		if action.Checkbox(c, "Loop", &p.loop, action.ChoiceOptions{ID: "loop"}).Changed() {
			pl.SetLoop(p.loop && !p.audio)
		}
	})
}
