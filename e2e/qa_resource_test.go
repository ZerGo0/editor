//go:build e2e

package e2e

import (
	"cmp"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/ZerGo0/editor/e2e/qa"
)

const (
	resourceReportName = "resources.json"
	tempDirEnv         = "TMPDIR"
	reportIndent       = "  "
	quietWindow        = 100 * time.Millisecond
	quietCPUPerWindow  = 5 * time.Millisecond
	quietWindows       = 4
	idleCPUWindow      = time.Second
	quietTimeout       = 5 * time.Second
)

type usageRecord struct {
	CPUMs         float64 `json:"cpuMs"`
	ChildCPUMs    float64 `json:"childCpuMs"`
	PeakFootprint uint64  `json:"lifetimePeakFootprintBytes"`
	DiskRead      uint64  `json:"diskReadBytes"`
	DiskWritten   uint64  `json:"diskWrittenBytes"`
}

type resourceRecord struct {
	Action          string           `json:"action"`
	Owner           string           `json:"owner"`
	Runs            int              `json:"runs"`
	SettledMedianMs float64          `json:"settledMedianMs"`
	SettledP95Ms    float64          `json:"settledP95Ms"`
	FirstFrameP95Ms float64          `json:"firstFrameP95Ms"`
	CPUMs           float64          `json:"cpuMs"`
	ChildCPUMs      float64          `json:"childCpuMs"`
	CPUPerRunMs     float64          `json:"cpuPerRunMs"`
	StartFootprint  uint64           `json:"startFootprintBytes"`
	PeakFootprint   uint64           `json:"peakFootprintBytes"`
	LifetimePeak    uint64           `json:"lifetimePeakFootprintBytes"`
	EndResident     uint64           `json:"endResidentBytes"`
	EndFootprint    uint64           `json:"endFootprintBytes"`
	FootprintGrowth int64            `json:"footprintGrowthBytes"`
	Fixture         string           `json:"fixture,omitempty"`
	FixtureBytes    int64            `json:"fixtureBytes,omitempty"`
	DiskRead        uint64           `json:"diskReadBytes"`
	DiskWritten     uint64           `json:"diskWrittenBytes"`
	IdleCPUMs       float64          `json:"idleCpuMs"`
	IdleWindowMs    float64          `json:"idleWindowMs"`
	IdleCPUShare    float64          `json:"idleCpuShare"`
	IdleFrames      int              `json:"idleFrames"`
	Startup         usageRecord      `json:"startup"`
	DataBytes       map[string]int64 `json:"dataBytes"`
	DataBytesTotal  int64            `json:"dataBytesTotal"`
}

func (q *QAApp) waitFor() time.Duration { return cmp.Or(q.Opts.WaitTimeout, navTimeout) }

func milliseconds(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }

func (q *QAApp) Usage() qa.Usage {
	q.t.Helper()
	u, err := qa.ReadUsage(q.cmd.Process.Pid)
	if err != nil {
		q.t.Fatal(err)
	}
	return u
}

func (q *QAApp) StartMeter() qa.Usage {
	q.t.Helper()
	if err := qa.ResetPeak(q.cmd.Process.Pid); err != nil {
		q.t.Fatal(err)
	}
	return q.Usage()
}

func (q *QAApp) MeasureIdle() (int, qa.Idle) {
	q.t.Helper()
	q.Settle()
	offset, before := q.outputLen(), q.Usage()
	time.Sleep(idleWatchWindow)
	frames := qa.CountFrames(q.outputFrom(offset))
	time.Sleep(idleCPUWindow - idleWatchWindow)
	after := q.Usage()
	return frames, qa.Idle{CPU: after.CPU + after.ChildCPU - before.CPU - before.ChildCPU, Window: idleCPUWindow}
}

func (q *QAApp) WaitCPUQuiet() {
	q.t.Helper()
	deadline := time.Now().Add(cmp.Or(q.Opts.QuietTimeout, quietTimeout))
	prev, quiet := q.Usage(), 0
	for time.Now().Before(deadline) {
		time.Sleep(quietWindow)
		cur := q.Usage()
		if cur.CPU+cur.ChildCPU-prev.CPU-prev.ChildCPU > quietCPUPerWindow || q.hasChildren() {
			quiet = 0
		} else if quiet++; quiet == quietWindows {
			return
		}
		prev = cur
	}
}

func (q *QAApp) hasChildren() bool {
	return exec.Command("pgrep", "-P", strconv.Itoa(q.cmd.Process.Pid)).Run() == nil
}

func startupRecord(u qa.Usage) usageRecord {
	return usageRecord{CPUMs: milliseconds(u.CPU), ChildCPUMs: milliseconds(u.ChildCPU), PeakFootprint: u.LifetimePeak, DiskRead: u.DiskRead, DiskWritten: u.DiskWritten}
}

func writeResourceReport(t *testing.T, records []resourceRecord) {
	t.Helper()
	data, err := json.MarshalIndent(records, "", reportIndent)
	if err != nil {
		t.Fatal(err)
	}
	dir := artifactDir()
	if dir == "" {
		dir = t.TempDir()
	}
	if err := os.MkdirAll(dir, artifactDirPerm); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, resourceReportName)
	if err := os.WriteFile(path, data, shotFilePerm); err != nil {
		t.Fatal(err)
	}
	t.Logf("resource report: %s", path)
}
