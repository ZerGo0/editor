package qa

import (
	"testing"
	"time"
)

func TestSummarize(t *testing.T) {
	var samples []time.Duration
	for i := 1; i <= 20; i++ {
		samples = append(samples, time.Duration(i)*time.Millisecond)
	}
	s := Summarize(samples)
	if s.N != 20 || s.Median != 10*time.Millisecond || s.P95 != 19*time.Millisecond || s.Max != 20*time.Millisecond || s.Min != time.Millisecond {
		t.Fatalf("stats = %+v", s)
	}
	if got := Summarize(nil); got.N != 0 {
		t.Fatalf("empty = %+v", got)
	}
}

func TestOverBudget(t *testing.T) {
	s := Stats{N: 5, Median: 10 * time.Millisecond, P95: 40 * time.Millisecond}
	if flags := OverBudget("typing", OwnerWorkspace, s, 50*time.Millisecond); len(flags) != 0 {
		t.Fatalf("flags = %v", flags)
	}
	if flags := OverBudget("typing", OwnerWorkspace, s, 30*time.Millisecond); len(flags) != 1 || flags[0].Rule != RuleBudget || flags[0].Owner != OwnerWorkspace {
		t.Fatalf("flags = %v", flags)
	}
}

const frameSample = `2026/11/26 11:00:00 mygo: frame 41 took 3.2 ms: build 1.1 (2 passes), layout 0.9, paint 0.8, present 0.2 (gpu), other 0.2; meanwhile the process made about 1 allocations
2026/11/26 11:00:01 unrelated line
2026/11/26 11:00:02 mygo: frame 42 took 12.5 ms: build 6.0, layout 2.0, paint 3.0, present 1.0 (cpu), other 0.5; meanwhile
`

func TestParseFrames(t *testing.T) {
	frames := ParseFrames(frameSample)
	if len(frames) != 2 || frames[0].Number != 41 || frames[0].Build != 1100*time.Microsecond || frames[1].Total != 12500*time.Microsecond {
		t.Fatalf("frames = %+v", frames)
	}
	if got := CountFrames(frameSample); got != 2 {
		t.Fatalf("count = %d", got)
	}
	if got := SlowestFrame(frames); got.Number != 42 {
		t.Fatalf("slowest = %+v", got)
	}
}

func TestIdleFlag(t *testing.T) {
	if flags := IdleFlag("typing", OwnerWorkspace, 0, time.Second); len(flags) != 0 {
		t.Fatalf("flags = %v", flags)
	}
	if flags := IdleFlag("typing", OwnerWorkspace, 3, time.Second); len(flags) != 1 || flags[0].Rule != RuleIdle {
		t.Fatalf("flags = %v", flags)
	}
}
