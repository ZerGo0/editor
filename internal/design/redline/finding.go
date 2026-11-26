package redline

import (
	"fmt"
	"sort"
	"strings"
)

type Rule string

const (
	RuleAlign    Rule = "align"
	RuleGap      Rule = "gap"
	RulePadding  Rule = "padding"
	RuleOptical  Rule = "optical"
	RuleText     Rule = "text-band"
	RuleCrisp    Rule = "crisp"
	RuleIconSet  Rule = "icon-set"
	RuleContrast Rule = "contrast"
)

const (
	UnitPx       = "px"
	UnitRatio    = ":1"
	UnitShare    = "share"
	UnitDevicePx = "device px"
	noBox        = -1
)

type Finding struct {
	Rule      Rule
	Box       int
	Other     int
	Check     string
	Measured  float64
	Expected  float64
	Tolerance float64
	Unit      string
	Fix       string
	Area      Rect
}

type MeasureKind string

const (
	MeasureIcon    MeasureKind = "icon"
	MeasureText    MeasureKind = "text"
	MeasureRowIcon MeasureKind = "row icon"
)

type Measure struct {
	Box      int
	Kind     MeasureKind
	Ink      Rect
	DX, DY   float64
	HasX     bool
	Text     bool
	Stroke   float64
	Soft     float64
	Contrast float64
	Tinted   bool
}

func (r Result) Key(f Finding) string {
	b := r.Boxes[f.Box]
	return fmt.Sprintf("%s %s %q %s", f.Rule, b.Role, b.Label, f.Check)
}

func (r Result) Unallowed(allow []string) []int {
	var out []int
	for n, f := range r.Findings {
		key := r.Key(f)
		ok := false
		for _, a := range allow {
			if a = strings.TrimSpace(a); a != "" && strings.HasPrefix(key, a) {
				ok = true
				break
			}
		}
		if !ok {
			out = append(out, n)
		}
	}
	return out
}

var ruleOrder = []Rule{RuleOptical, RuleText, RuleAlign, RuleGap, RulePadding, RuleIconSet, RuleCrisp, RuleContrast}

func ruleRank(r Rule) int {
	for i, x := range ruleOrder {
		if x == r {
			return i
		}
	}
	return len(ruleOrder)
}

func sortFindings(fs []Finding) {
	sort.SliceStable(fs, func(i, j int) bool {
		a, b := fs[i], fs[j]
		if ra, rb := ruleRank(a.Rule), ruleRank(b.Rule); ra != rb {
			return ra < rb
		}
		if a.Area.Y != b.Area.Y {
			return a.Area.Y < b.Area.Y
		}
		if a.Area.X != b.Area.X {
			return a.Area.X < b.Area.X
		}
		if a.Box != b.Box {
			return a.Box < b.Box
		}
		return a.Check < b.Check
	})
}
