//go:build e2e

package e2e

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ZerGo0/editor/e2e/qa"
)

const (
	ruleActionFailed = "action-failed"
	leakSettleWait   = 3 * time.Second
)

type latencyRun struct {
	demo    DemoProject
	mu      sync.Mutex
	rows    map[int]string
	flags   []qa.Flag
	records map[int]resourceRecord
}

func (r *latencyRun) flag(f ...qa.Flag) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.flags = append(r.flags, f...)
}

func newLatencyRun(demo DemoProject) *latencyRun {
	return &latencyRun{demo: demo, rows: map[int]string{}, records: map[int]resourceRecord{}}
}

func (r *latencyRun) failure(action latencyAction, elapsed time.Duration) qa.Flag {
	return qa.Flag{Rule: ruleActionFailed, View: action.Name, Owner: action.Owner, Role: "action", Label: action.Name,
		Measured: fmt.Sprintf("failed or timed out after %s", elapsed.Round(time.Second)), Expected: "the scenario completes within its timeout", Fix: "read the test log for the failing step; the app may hang or run out of memory at this size"}
}

func (r *latencyRun) action(t *testing.T, index int, action latencyAction) {
	demo := r.demo
	if action.Project != nil {
		demo = action.Project(t)
	}
	budget, ok := actionBudgets[action.Name]
	if !ok {
		t.Fatalf("no budget for %q in actionBudgets", action.Name)
	}
	var opts QAOptions
	if action.Options != nil {
		opts = action.Options(demo)
	} else {
		opts = defaultLatencyOptions(demo)
	}
	launched := time.Now()
	defer func() {
		if t.Failed() {
			r.flag(r.failure(action, time.Since(launched)))
		}
	}()
	q := LaunchQA(t, demo.Root, opts)
	if action.Timeout > 0 {
		watchdog := time.AfterFunc(action.Timeout, func() { _ = q.cmd.Process.Kill() })
		defer watchdog.Stop()
	}
	startupWall := time.Since(launched)
	startup := q.Usage()
	if action.Prepare != nil {
		if err := action.Prepare(q, demo); err != nil {
			t.Fatalf("prepare: %v", err)
		}
	}
	maxWait := cmp.Or(action.MaxWait, burstMax)
	warmups := 1
	if action.Cold {
		warmups = 0
	}
	runs := action.Samples + warmups
	if action.Samples == 0 {
		runs = 0
	}
	var settled, first []time.Duration
	var frameCounts []int
	var slowest qa.Frame
	begin := q.StartMeter()
	for i := range runs {
		if action.Reset != nil {
			action.Reset(q, demo, i)
		}
		b := q.Burst(action.Quiet, maxWait, func() { action.Do(q, demo, i) })
		if i < warmups {
			continue
		}
		if len(b.Frames) == 0 {
			r.flag(qa.Flag{Rule: rulePendingFrame, View: action.Name, Owner: action.Owner, Role: "action", Label: action.Name, Measured: "no frame drawn after the input", Expected: "the input draws a frame", Fix: "check that the action reaches the UI"})
			continue
		}
		if b.Continuous {
			r.flag(qa.Flag{Rule: ruleFrameStorm, View: action.Name, Owner: action.Owner, Role: "action", Label: action.Name, Measured: fmt.Sprintf("%d frames in %s after one input, still drawing", len(b.Frames), maxWait), Expected: "the input settles within a few frames (no continuous redraw)", Fix: "find the view that requests a frame every frame (scroll-into-view retry, After loop, Animate) and make it converge"})
		}
		settled, first = append(settled, b.Settled), append(first, b.First)
		frameCounts = append(frameCounts, len(b.Frames))
		if s := qa.SlowestFrame(b.Frames); s.Total > slowest.Total {
			slowest = s
		}
	}
	if budget.leak > 0 {
		time.Sleep(leakSettleWait)
	}
	q.WaitCPUQuiet()
	end := q.Usage()
	work := qa.Between(begin, end, runs)
	if action.Samples == 0 {
		work = qa.Between(qa.Usage{}, startup, 1)
		settled, first, frameCounts = []time.Duration{startupWall}, []time.Duration{startupWall}, []int{0}
	}
	if len(frameCounts) == 0 {
		return
	}
	stats, firstStats := qa.Summarize(settled), qa.Summarize(first)
	idle, quiet := q.MeasureIdle()
	data := qa.DataBytes(q.UserData, q.Temp)
	if action.Verify != nil {
		if err := action.Verify(q, demo); err != nil {
			t.Fatalf("verify: %v", err)
		}
	}
	growth := int64(end.Footprint) - int64(begin.Footprint)
	record := resourceRecord{
		Action: action.Name, Owner: action.Owner, Runs: work.Runs, Fixture: action.Fixture, FixtureBytes: stressFixtureBytes(action.Fixture),
		SettledMedianMs: milliseconds(stats.Median), SettledP95Ms: milliseconds(stats.P95), FirstFrameP95Ms: milliseconds(firstStats.P95),
		CPUMs: milliseconds(work.CPU), ChildCPUMs: milliseconds(work.ChildCPU), CPUPerRunMs: milliseconds(work.CPUPerRun()),
		StartFootprint: work.StartFootprint, PeakFootprint: work.PeakFootprint, LifetimePeak: q.Usage().LifetimePeak, EndResident: work.EndResident,
		EndFootprint: end.Footprint, FootprintGrowth: growth,
		DiskRead: work.DiskRead, DiskWritten: work.DiskWritten,
		IdleCPUMs: milliseconds(quiet.CPU), IdleWindowMs: milliseconds(quiet.Window), IdleCPUShare: quiet.Share(), IdleFrames: idle,
		Startup: startupRecord(startup), DataBytes: data, DataBytesTotal: qa.TotalBytes(data),
	}
	r.flag(qa.OverResources(action.Name, action.Owner, work, quiet, qa.TotalBytes(data), budget.resources())...)
	r.flag(qa.OverStartup(action.Owner, startup, budget.startup())...)
	r.flag(qa.OverGrowth(action.Name, action.Owner, growth, budget.leak)...)
	row := fmt.Sprintf("%-52s %-44s %-44s %-12d %s / %d\n", action.Name, stats, firstStats, frameCounts[len(frameCounts)/2], slowest.Total.Round(time.Microsecond*100), idle)
	r.mu.Lock()
	r.rows[index], r.records[index] = row, record
	r.mu.Unlock()
	r.flag(qa.OverBudget(action.Name, action.Owner, stats, budget.latency)...)
	r.flag(qa.OverBudget(action.Name+" (first frame)", action.Owner, firstStats, budget.firstFrame)...)
	r.flag(qa.IdleFlag(action.Name, action.Owner, max(idle-action.IdleFrames, 0), idleWatchWindow)...)
	q.Report("latency-"+strings.ReplaceAll(action.Name, " ", "-"), stats.String()+"\n"+firstStats.String())
}

func (r *latencyRun) finish(t *testing.T) {
	t.Helper()
	var report strings.Builder
	fmt.Fprintf(&report, "%-52s %-44s %-44s %-12s %s\n", "action", "input to last frame", "input to first frame", "frames", "slowest frame / idle frames")
	indexes := slices.Sorted(maps.Keys(r.rows))
	records := make([]resourceRecord, 0, len(indexes))
	for _, i := range indexes {
		report.WriteString(r.rows[i])
		records = append(records, r.records[i])
	}
	t.Logf("\n%s", report.String())
	if dir := artifactDir(); dir != "" {
		writeReport(t, "latency", report.String())
	}
	writeResourceReport(t, records)
	if len(r.flags) > 0 {
		t.Errorf("%d flags:\n%s", len(qa.Dedupe(r.flags)), qa.Format(qa.Dedupe(r.flags)))
	}
}
