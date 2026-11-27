package qa

import (
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

const (
	percentile95       = 0.95
	percentile50       = 0.5
	frameMarker        = "mygo: frame "
	frameRepaintMarker = "(painted again)"
	RuleBudget         = "latency-budget"
	RuleIdle           = "idle-frames"
)

type Stats struct {
	N      int
	Median time.Duration
	P95    time.Duration
	Max    time.Duration
	Min    time.Duration
}

func Summarize(samples []time.Duration) Stats {
	if len(samples) == 0 {
		return Stats{}
	}
	sorted := slices.Clone(samples)
	slices.Sort(sorted)
	return Stats{N: len(sorted), Median: Percentile(sorted, percentile50), P95: Percentile(sorted, percentile95), Max: sorted[len(sorted)-1], Min: sorted[0]}
}

func Percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	rank := int(math.Ceil(p*float64(len(sorted)))) - 1
	return sorted[min(max(rank, 0), len(sorted)-1)]
}

func (s Stats) String() string {
	return fmt.Sprintf("n=%d median=%s p95=%s max=%s", s.N, s.Median.Round(time.Millisecond/10), s.P95.Round(time.Millisecond/10), s.Max.Round(time.Millisecond/10))
}

func OverBudget(action, owner string, s Stats, budget time.Duration) []Flag {
	if s.P95 <= budget && s.Median <= budget {
		return nil
	}
	return []Flag{{Rule: RuleBudget, View: action, Owner: owner, Role: "action", Label: action,
		Measured: fmt.Sprintf("median %s, p95 %s", s.Median.Round(time.Millisecond/10), s.P95.Round(time.Millisecond/10)),
		Expected: "p95 within " + budget.String(), Fix: "profile the frame with MYGO_FRAME_STATS=all and remove work from build (cache, virtualize, move off the UI thread)"}}
}

type Frame struct {
	Repaint bool
	Number  int
	Total   time.Duration
	Build   time.Duration
	Layout  time.Duration
	Paint   time.Duration
	Present time.Duration
}

var frameLine = regexp.MustCompile(`mygo: frame (\d+) took ([0-9.]+) ms: build ([0-9.]+)(?: \([^)]*\))?, layout ([0-9.]+), paint ([0-9.]+), present ([0-9.]+)`)

func ParseFrames(output string) []Frame {
	var frames []Frame
	for line := range strings.SplitSeq(output, "\n") {
		if !strings.Contains(line, frameMarker) {
			continue
		}
		m := frameLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		num, _ := strconv.Atoi(m[1])
		frames = append(frames, Frame{Repaint: strings.Contains(line, frameRepaintMarker), Number: num, Total: ms(m[2]), Build: ms(m[3]), Layout: ms(m[4]), Paint: ms(m[5]), Present: ms(m[6])})
	}
	return frames
}

func ms(s string) time.Duration {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return time.Duration(v * float64(time.Millisecond))
}

func CountFrames(output string) int { return strings.Count(output, frameMarker) }

func CountBuiltFrames(output string) int {
	return CountFrames(output) - strings.Count(output, frameRepaintMarker)
}

func Built(frames []Frame) []Frame {
	return slices.DeleteFunc(slices.Clone(frames), func(f Frame) bool { return f.Repaint })
}

func SlowestFrame(frames []Frame) Frame {
	var slowest Frame
	for _, f := range frames {
		if f.Total > slowest.Total {
			slowest = f
		}
	}
	return slowest
}

func IdleFlag(action, owner string, frames int, window time.Duration) []Flag {
	if frames == 0 {
		return nil
	}
	return []Flag{{Rule: RuleIdle, View: action, Owner: owner, Role: "action", Label: action,
		Measured: fmt.Sprintf("%d frames drawn in %s with no input", frames, window), Expected: "0 frames while idle (only progress indicators and the caret blink may draw)", Fix: "remove timers or After loops that redraw while the view is static"}}
}
