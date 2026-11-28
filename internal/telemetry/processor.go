package telemetry

import (
	"context"
	"time"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

const exportThreshold = 25 * time.Millisecond

type processor struct {
	p    *pipeline
	next sdktrace.SpanProcessor
}

func (x *processor) OnStart(context.Context, sdktrace.ReadWriteSpan) {}

func (x *processor) OnEnd(s sdktrace.ReadOnlySpan) {
	d := s.EndTime().Sub(s.StartTime())
	ms := millis(d)
	x.p.recordSpan(s.Name(), ms)
	if x.p.check(s.Name(), ms) || d >= exportThreshold {
		x.next.OnEnd(s)
	}
	x.p.collectIfDue()
}

func (x *processor) Shutdown(ctx context.Context) error { return x.next.Shutdown(ctx) }

func (x *processor) ForceFlush(ctx context.Context) error { return x.next.ForceFlush(ctx) }

func millis(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }
