package git

import (
	"regexp"
	"strconv"
)

type phaseSpan struct{ from, to float64 }

var (
	progressLine = regexp.MustCompile(`^(?:remote: )?([A-Za-z][A-Za-z ]*?):\s+(\d{1,3})%`)
	phaseSpans   = map[string]phaseSpan{
		"Counting objects":    {0, 0.1},
		"Compressing objects": {0.1, 0.2},
		"Receiving objects":   {0.2, 0.7},
		"Writing objects":     {0.2, 0.7},
		"Resolving deltas":    {0.7, 0.9},
		"Updating files":      {0.9, 1},
	}
)

const percentScale = 100

func ParseProgress(line string) (phase string, done float64, ok bool) {
	m := progressLine.FindStringSubmatch(line)
	if m == nil {
		return "", 0, false
	}
	pct, err := strconv.Atoi(m[2])
	if err != nil || pct > percentScale {
		return "", 0, false
	}
	frac := float64(pct) / percentScale
	span, known := phaseSpans[m[1]]
	if !known {
		span = phaseSpan{0, 1}
	}
	return m[1] + " " + m[2] + "%", span.from + (span.to-span.from)*frac, true
}
