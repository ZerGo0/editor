package telemetry

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ZerGo0/editor/internal/applog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutlog"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	otellog "go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

const (
	DirName     = "telemetry"
	TracesFile  = "traces.jsonl"
	MetricsFile = "metrics.jsonl"
	LogsFile    = "logs.jsonl"

	maxFileSize     = 2 << 20
	maxBackups      = 1
	scopeName       = "github.com/ZerGo0/editor"
	serviceName     = "editor"
	collectInterval = time.Minute
)

type pipeline struct {
	traces  *sdktrace.TracerProvider
	meters  *sdkmetric.MeterProvider
	reader  *sdkmetric.ManualReader
	metrics sdkmetric.Exporter
	logs    *sdklog.LoggerProvider
	logger  otellog.Logger
	inst    instruments
	budgets budgetLog
	process processWindow

	collectMu   sync.Mutex
	lastCollect atomic.Int64
	collecting  atomic.Bool
}

var (
	current atomic.Pointer[pipeline]
	tracer  = otel.Tracer(scopeName)
)

func Setup(logsDir, version string) error {
	dir := filepath.Join(logsDir, DirName)
	open := func(name string) (*applog.Writer, error) {
		return applog.Open(filepath.Join(dir, name), maxFileSize, maxBackups)
	}
	traceFile, err := open(TracesFile)
	if err != nil {
		return err
	}
	metricFile, err := open(MetricsFile)
	if err != nil {
		return err
	}
	logFile, err := open(LogsFile)
	if err != nil {
		return err
	}
	traceExp, err := stdouttrace.New(stdouttrace.WithWriter(traceFile))
	if err != nil {
		return fmt.Errorf("trace exporter: %w", err)
	}
	metricExp, err := stdoutmetric.New(stdoutmetric.WithWriter(metricFile))
	if err != nil {
		return fmt.Errorf("metric exporter: %w", err)
	}
	logExp, err := stdoutlog.New(stdoutlog.WithWriter(logFile))
	if err != nil {
		return fmt.Errorf("log exporter: %w", err)
	}
	now := time.Now()
	res := resource.NewSchemaless(
		attribute.String("service.name", serviceName),
		attribute.String("service.version", version),
		attribute.String("session.id", strconv.FormatInt(now.UnixMilli(), 10)),
	)
	p := &pipeline{reader: sdkmetric.NewManualReader(), metrics: metricExp}
	p.meters = sdkmetric.NewMeterProvider(sdkmetric.WithResource(res), sdkmetric.WithReader(p.reader))
	p.logs = sdklog.NewLoggerProvider(sdklog.WithResource(res), sdklog.WithProcessor(sdklog.NewSimpleProcessor(logExp)))
	p.logger = p.logs.Logger(scopeName)
	p.process.start(now)
	if err := p.instrument(p.meters.Meter(scopeName)); err != nil {
		return err
	}
	p.traces = sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(&processor{p: p, next: sdktrace.NewSimpleSpanProcessor(traceExp)}),
	)
	p.lastCollect.Store(now.UnixNano())
	otel.SetTracerProvider(p.traces)
	current.Store(p)
	return nil
}

func Start(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	return tracer.Start(ctx, name, trace.WithAttributes(attrs...))
}

func Collect() {
	if p := current.Load(); p != nil {
		p.collect()
	}
}

func CollectIfDue() {
	if p := current.Load(); p != nil {
		p.collectIfDue()
	}
}

func (p *pipeline) collectIfDue() {
	if time.Since(time.Unix(0, p.lastCollect.Load())) < collectInterval || !p.collecting.CompareAndSwap(false, true) {
		return
	}
	applog.Go(func() {
		defer p.collecting.Store(false)
		p.collect()
	})
}

func (p *pipeline) collect() {
	p.collectMu.Lock()
	defer p.collectMu.Unlock()
	ctx := context.Background()
	var rm metricdata.ResourceMetrics
	if err := p.reader.Collect(ctx, &rm); err != nil {
		log.Printf("telemetry: collect metrics: %v", err)
		return
	}
	if err := p.metrics.Export(ctx, &rm); err != nil {
		log.Printf("telemetry: export metrics: %v", err)
	}
	p.lastCollect.Store(time.Now().UnixNano())
}
