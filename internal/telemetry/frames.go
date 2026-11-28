package telemetry

import (
	"context"
	"time"

	"github.com/egoist/mygo/ui"
)

type FrameClock struct {
	start        time.Time
	building     bool
	input        time.Time
	inputInBuild bool
}

func (f *FrameClock) BuildStarted() {
	if current.Load() == nil {
		return
	}
	if f.building && f.inputInBuild {
		f.input = time.Time{}
	}
	f.start, f.building = time.Now(), true
}

func (f *FrameClock) Input(ev ui.InputEvent) {
	if current.Load() == nil || !f.input.IsZero() || !showsInput(ev.Kind) {
		return
	}
	f.input = ev.Time
	if f.input.IsZero() {
		f.input = time.Now()
	}
	f.inputInBuild = f.building
}

func (f *FrameClock) Painted() {
	p := current.Load()
	if p == nil {
		return
	}
	now := time.Now()
	ctx := context.Background()
	if f.building {
		f.building = false
		p.inst.frame.Record(ctx, millis(now.Sub(f.start)))
	}
	if f.input.IsZero() {
		return
	}
	latency := millis(now.Sub(f.input))
	f.input = time.Time{}
	p.inst.input.Record(ctx, latency)
	p.check(actionInputLatency, latency)
}

func showsInput(k ui.InputKind) bool {
	switch k {
	case ui.InputKeyDown, ui.InputText, ui.InputPointerDown, ui.InputScroll:
		return true
	}
	return false
}
