package telemetry

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"go.opentelemetry.io/otel/trace"
)

const testSlowSpan = "test.slow"

func setupTest(t *testing.T) string {
	t.Helper()
	logs := t.TempDir()
	if err := Setup(logs, "test"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { current.Store(nil) })
	return filepath.Join(logs, DirName)
}

func readLines(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.FieldsFunc(string(data), func(r rune) bool { return r == '\n' })
}

func spanFor(name string, d time.Duration) {
	start := time.Now()
	_, s := tracer.Start(context.Background(), name, trace.WithTimestamp(start))
	s.End(trace.WithTimestamp(start.Add(d)))
}

func TestSpansReachTheTraceFileOnlyWhenSlowOrOverBudget(t *testing.T) {
	dir := setupTest(t)
	Budgets[testSlowSpan] = Budget{Limit: 1, Unit: unitMs}
	t.Cleanup(func() { delete(Budgets, testSlowSpan) })
	spanFor("test.fast", time.Millisecond)
	spanFor(SpanFileLoad, exportThreshold)
	spanFor(testSlowSpan, 2*time.Millisecond)
	spanFor(testSlowSpan, 3*time.Millisecond)

	traces := readLines(t, filepath.Join(dir, TracesFile))
	var names []string
	for _, line := range traces {
		var span struct{ Name string }
		if err := json.Unmarshal([]byte(line), &span); err != nil {
			t.Fatalf("trace line is not JSON: %v", err)
		}
		names = append(names, span.Name)
	}
	if got, want := strings.Join(names, " "), SpanFileLoad+" "+testSlowSpan+" "+testSlowSpan; got != want {
		t.Fatalf("traced spans = %q, want %q", got, want)
	}
	logs := readLines(t, filepath.Join(dir, LogsFile))
	if len(logs) != 1 || !strings.Contains(logs[0], budgetEvent) || !strings.Contains(logs[0], testSlowSpan) {
		t.Fatalf("logs = %q, want one %s record for %s", logs, budgetEvent, testSlowSpan)
	}
}

func TestCollectWritesOneMetricsLineWithFramesAndProcess(t *testing.T) {
	dir := setupTest(t)
	var f FrameClock
	f.BuildStarted()
	f.Input(ui.InputEvent{Kind: ui.InputKeyDown, Time: time.Now()})
	f.Painted()
	f.BuildStarted()
	f.Input(ui.InputEvent{Kind: ui.InputPointerMove, Time: time.Now()})
	f.Painted()
	Collect()

	lines := readLines(t, filepath.Join(dir, MetricsFile))
	if len(lines) != 1 {
		t.Fatalf("metrics has %d lines, want 1", len(lines))
	}
	counts := histogramCounts(t, lines[0])
	if counts["editor.frame.duration"] != 2 || counts["editor.input.latency"] != 1 {
		t.Fatalf("histogram counts = %v, want 2 frames and 1 input", counts)
	}
	for _, name := range []string{"editor.process.cpu.time", "editor.process.memory.peak_rss", "editor.go.heap", "editor.disk.written"} {
		if !strings.Contains(lines[0], name) {
			t.Errorf("metrics lack %s", name)
		}
	}
}

func TestFrameClockDropsInputWhenItsBuildPaintsNothing(t *testing.T) {
	dir := setupTest(t)
	var f FrameClock
	f.BuildStarted()
	f.Input(ui.InputEvent{Kind: ui.InputText, Time: time.Now()})
	f.BuildStarted()
	f.Painted()
	Collect()
	counts := histogramCounts(t, readLines(t, filepath.Join(dir, MetricsFile))[0])
	if counts["editor.input.latency"] != 0 {
		t.Fatalf("input latency count = %d, want 0", counts["editor.input.latency"])
	}
}

func histogramCounts(t *testing.T, line string) map[string]int {
	t.Helper()
	var rm struct {
		ScopeMetrics []struct {
			Metrics []struct {
				Name string
				Data struct {
					DataPoints []struct{ Count int }
				}
			}
		}
	}
	if err := json.Unmarshal([]byte(line), &rm); err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			for _, dp := range m.Data.DataPoints {
				counts[m.Name] += dp.Count
			}
		}
	}
	return counts
}
