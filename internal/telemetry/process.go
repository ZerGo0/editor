package telemetry

import (
	"runtime/metrics"
	"sync"
	"syscall"
	"time"

	"github.com/ZerGo0/editor/internal/fsutil"
)

const (
	heapMetric    = "/memory/classes/heap/objects:bytes"
	minRateWindow = 10 * time.Second
)

type processSample struct {
	at      time.Time
	cpu     time.Duration
	peakRSS uint64
	heap    uint64
	written int64
}

type processWindow struct {
	mu   sync.Mutex
	last processSample
}

func (w *processWindow) start(now time.Time) {
	w.last = readProcess(now)
}

func (w *processWindow) sample() processSample {
	return readProcess(time.Now())
}

func (w *processWindow) advance(s processSample) processSample {
	w.mu.Lock()
	defer w.mu.Unlock()
	prev := w.last
	w.last = s
	return prev
}

func readProcess(now time.Time) processSample {
	var ru syscall.Rusage
	_ = syscall.Getrusage(syscall.RUSAGE_SELF, &ru)
	heap := []metrics.Sample{{Name: heapMetric}}
	metrics.Read(heap)
	s := processSample{
		at:      now,
		cpu:     time.Duration(ru.Utime.Nano() + ru.Stime.Nano()),
		peakRSS: uint64(ru.Maxrss),
		written: fsutil.Written(),
	}
	if heap[0].Value.Kind() == metrics.KindUint64 {
		s.heap = heap[0].Value.Uint64()
	}
	return s
}

func (p *pipeline) checkProcess(s processSample) {
	prev := p.process.advance(s)
	p.check(actionHeap, float64(s.heap))
	p.check(actionPeakRSS, float64(s.peakRSS))
	wall := s.at.Sub(prev.at)
	if wall < minRateWindow {
		return
	}
	p.check(actionCPU, float64(s.cpu-prev.cpu)/float64(wall))
	p.check(actionDiskRate, float64(s.written-prev.written)/wall.Minutes())
}
