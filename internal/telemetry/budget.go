package telemetry

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	otellog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/metric"
)

const (
	SpanTextSearch = "search.text"
	SpanIndex      = "search.index"
	SpanFileLoad   = "file.load"
	SpanGitPrefix  = "git."

	actionGit          = "git"
	actionGitNetwork   = "git.network"
	actionInputLatency = "input.latency"
	actionCPU          = "cpu.share"
	actionHeap         = "memory.heap"
	actionPeakRSS      = "memory.peak_rss"
	actionDiskRate     = "disk.write_rate"

	unitShare       = "1"
	unitBytesPerMin = "By/min"
	budgetEvent     = "budget.exceeded"
	mib             = 1 << 20
)

type Budget struct {
	Limit float64
	Unit  string
}

var Budgets = map[string]Budget{
	SpanTextSearch:     {Limit: 3000, Unit: unitMs},
	SpanIndex:          {Limit: 10000, Unit: unitMs},
	SpanFileLoad:       {Limit: 1000, Unit: unitMs},
	actionGit:          {Limit: 5000, Unit: unitMs},
	actionGitNetwork:   {Limit: 60000, Unit: unitMs},
	actionInputLatency: {Limit: 100, Unit: unitMs},
	actionCPU:          {Limit: 0.5, Unit: unitShare},
	actionHeap:         {Limit: 512 * mib, Unit: unitBytes},
	actionPeakRSS:      {Limit: 1024 * mib, Unit: unitBytes},
	actionDiskRate:     {Limit: 32 * mib, Unit: unitBytesPerMin},
}

var gitNetwork = map[string]bool{"fetch": true, "pull": true, "push": true, "clone": true, "ls-remote": true}

func budgetKey(action string) string {
	if sub, ok := strings.CutPrefix(action, SpanGitPrefix); ok {
		if gitNetwork[sub] {
			return actionGitNetwork
		}
		return actionGit
	}
	return action
}

type budgetLog struct {
	mu   sync.Mutex
	seen map[string]bool
}

func (b *budgetLog) first(key string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.seen[key] {
		return false
	}
	if b.seen == nil {
		b.seen = map[string]bool{}
	}
	b.seen[key] = true
	return true
}

func (p *pipeline) check(action string, value float64) bool {
	key := budgetKey(action)
	b, ok := Budgets[key]
	if !ok || value <= b.Limit {
		return false
	}
	ctx := context.Background()
	p.inst.exceeded.Add(ctx, 1, metric.WithAttributes(attribute.String(actionKey, key)))
	if !p.budgets.first(key) {
		return true
	}
	msg := fmt.Sprintf("budget: %s at %s %s, over the budget of %s %s; later overruns only count in editor.budget.exceeded", action, formatValue(value), b.Unit, formatValue(b.Limit), b.Unit)
	var r otellog.Record
	r.SetEventName(budgetEvent)
	r.SetTimestamp(time.Now())
	r.SetSeverity(otellog.SeverityWarn)
	r.SetSeverityText("WARN")
	r.SetBody(attribute.StringValue(msg))
	r.AddAttributes(
		attribute.String(actionKey, key),
		attribute.String(spanKey, action),
		attribute.Float64("value", value),
		attribute.Float64("budget", b.Limit),
		attribute.String("unit", b.Unit),
	)
	p.logger.Emit(ctx, r)
	log.Print(msg)
	return true
}

func formatValue(v float64) string {
	return strings.TrimSuffix(strings.TrimRight(fmt.Sprintf("%.3f", v), "0"), ".")
}
