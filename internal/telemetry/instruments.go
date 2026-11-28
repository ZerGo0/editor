package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const (
	spanKey   = "span"
	actionKey = "action"
	unitMs    = "ms"
	unitBytes = "By"
	unitSec   = "s"
)

var latencyBounds = []float64{1, 2, 4, 8, 12, 16, 24, 33, 50, 75, 100, 150, 250, 500, 1000, 2500, 5000, 10000}

type instruments struct {
	frame    metric.Float64Histogram
	input    metric.Float64Histogram
	span     metric.Float64Histogram
	exceeded metric.Int64Counter
}

func (p *pipeline) instrument(m metric.Meter) error {
	var err error
	hist := func(name, desc string) metric.Float64Histogram {
		if err != nil {
			return nil
		}
		var h metric.Float64Histogram
		h, err = m.Float64Histogram(name, metric.WithDescription(desc), metric.WithUnit(unitMs), metric.WithExplicitBucketBoundaries(latencyBounds...))
		return h
	}
	p.inst.frame = hist("editor.frame.duration", "Time from the start of a view build to its paint")
	p.inst.input = hist("editor.input.latency", "Time from an input event to the paint of the frame that shows it")
	p.inst.span = hist("editor.span.duration", "Duration of each traced operation, by span name")
	if err != nil {
		return fmt.Errorf("telemetry instruments: %w", err)
	}
	if p.inst.exceeded, err = m.Int64Counter("editor.budget.exceeded", metric.WithDescription("Times an action went over its budget")); err != nil {
		return fmt.Errorf("telemetry instruments: %w", err)
	}
	cpu, err := m.Float64ObservableCounter("editor.process.cpu.time", metric.WithDescription("User and system CPU time of the process"), metric.WithUnit(unitSec))
	if err != nil {
		return fmt.Errorf("telemetry instruments: %w", err)
	}
	rss, err := m.Int64ObservableGauge("editor.process.memory.peak_rss", metric.WithDescription("Peak resident memory of the process"), metric.WithUnit(unitBytes))
	if err != nil {
		return fmt.Errorf("telemetry instruments: %w", err)
	}
	heap, err := m.Int64ObservableGauge("editor.go.heap", metric.WithDescription("Bytes of live and unswept heap objects"), metric.WithUnit(unitBytes))
	if err != nil {
		return fmt.Errorf("telemetry instruments: %w", err)
	}
	written, err := m.Int64ObservableCounter("editor.disk.written", metric.WithDescription("Bytes the app wrote through its own writers"), metric.WithUnit(unitBytes))
	if err != nil {
		return fmt.Errorf("telemetry instruments: %w", err)
	}
	_, err = m.RegisterCallback(func(_ context.Context, o metric.Observer) error {
		s := p.process.sample()
		o.ObserveFloat64(cpu, s.cpu.Seconds())
		o.ObserveInt64(rss, int64(s.peakRSS))
		o.ObserveInt64(heap, int64(s.heap))
		o.ObserveInt64(written, s.written)
		p.checkProcess(s)
		return nil
	}, cpu, rss, heap, written)
	if err != nil {
		return fmt.Errorf("telemetry callback: %w", err)
	}
	return nil
}

func (p *pipeline) recordSpan(name string, ms float64) {
	p.inst.span.Record(context.Background(), ms, metric.WithAttributes(attribute.String(spanKey, name)))
}
