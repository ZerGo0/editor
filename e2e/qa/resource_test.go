package qa

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

const (
	testMiB    = 1 << 20
	testIdle   = 600 * time.Millisecond
	testBudget = 50 * time.Millisecond
)

func testWork() Work {
	return Between(Usage{CPU: time.Second, Footprint: 100 * testMiB, DiskRead: 10, DiskWritten: 20},
		Usage{CPU: time.Second + 200*time.Millisecond, ChildCPU: 100 * time.Millisecond, IntervalPeak: 120 * testMiB, Resident: 110 * testMiB, DiskRead: 1010, DiskWritten: 20}, 10)
}

func TestBetweenDividesPerRun(t *testing.T) {
	w := testWork()
	if w.CPU != 200*time.Millisecond || w.ChildCPU != 100*time.Millisecond || w.CPUPerRun() != 30*time.Millisecond {
		t.Fatalf("work = %+v, per run %s", w, w.CPUPerRun())
	}
	if w.DiskRead != 1000 || w.DiskWritten != 0 || w.PeakFootprint != 120*testMiB || w.StartFootprint != 100*testMiB {
		t.Fatalf("work = %+v", w)
	}
}

func TestOverResources(t *testing.T) {
	ok := ResourceBudget{CPUPerRun: testBudget, PeakFootprint: 130 * testMiB, DiskReadPerRun: 200, DiskWrittenPerRun: 1, IdleShare: 0.01, DataBytes: 100}
	idle := Idle{CPU: testIdle / 200, Window: testIdle}
	if flags := OverResources("scroll", OwnerWorkspace, testWork(), idle, 50, ok); len(flags) != 0 {
		t.Fatalf("flags = %v", flags)
	}
	tight := ResourceBudget{CPUPerRun: time.Millisecond, PeakFootprint: testMiB, DiskReadPerRun: 1, DiskWrittenPerRun: 1, IdleShare: 0.0001, DataBytes: 1}
	flags := OverResources("scroll", OwnerWorkspace, testWork(), idle, 50, tight)
	rules := map[string]bool{}
	for _, f := range flags {
		rules[f.Rule] = true
	}
	for _, rule := range []string{RuleCPU, RuleMemory, RuleDiskIO, RuleIdleCPU, RuleDataSize} {
		if !rules[rule] {
			t.Errorf("missing flag %s in %v", rule, flags)
		}
	}
}

func TestOverStartup(t *testing.T) {
	u := Usage{CPU: 300 * time.Millisecond, ChildCPU: 50 * time.Millisecond, LifetimePeak: 130 * testMiB}
	if flags := OverStartup(OwnerWorkspace, u, StartupBudget{CPU: time.Second, PeakFootprint: 192 * testMiB}); len(flags) != 0 {
		t.Fatalf("flags = %v", flags)
	}
	flags := OverStartup(OwnerWorkspace, u, StartupBudget{CPU: 100 * time.Millisecond, PeakFootprint: 100 * testMiB})
	if len(flags) != 2 || flags[0].Rule != RuleStartup || flags[0].View != flags[1].View {
		t.Fatalf("flags = %v", flags)
	}
}

func TestIdleShare(t *testing.T) {
	if got := (Idle{CPU: 6 * time.Millisecond, Window: testIdle}).Share(); got != 0.01 {
		t.Fatalf("share = %v", got)
	}
	if got := (Idle{CPU: time.Millisecond}).Share(); got != 0 {
		t.Fatalf("share without a window = %v", got)
	}
}

func TestDataCategory(t *testing.T) {
	cases := map[string]string{
		"settings.json":              DataConfig,
		"logs/editor.log":            DataLogs,
		"data/history/blobs/ab/abcd": DataHistory,
		"data/backups/x.meta":        DataBackups,
		"data/shelf/1234/patch":      DataShelf,
		"data/other.bin":             DataOther,
		"data/unknown/file":          DataOther,
		"issues/abcd.json":           DataOther,
	}
	for rel, want := range cases {
		if got := DataCategory(filepath.FromSlash(rel)); got != want {
			t.Errorf("%s: category %s, want %s", rel, got, want)
		}
	}
}

func TestDataBytesSumsEachCategory(t *testing.T) {
	user, temp := t.TempDir(), t.TempDir()
	files := map[string]int{
		filepath.Join(user, "settings.json"):                 3,
		filepath.Join(user, "logs", "editor.log"):            5,
		filepath.Join(user, "data", "history", "blobs", "a"): 7,
		filepath.Join(temp, "sips-1"):                        11,
	}
	for path, size := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	sizes := DataBytes(user, temp)
	if sizes[DataConfig] != 3 || sizes[DataLogs] != 5 || sizes[DataHistory] != 7 || sizes[DataTemp] != 11 || TotalBytes(sizes) != 26 {
		t.Fatalf("sizes = %v", sizes)
	}
	if got := DataBytes("", ""); len(got) != 0 {
		t.Fatalf("empty roots = %v", got)
	}
}
