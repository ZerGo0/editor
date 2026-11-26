package viewer

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	byteUnit      = 1024
	thousandsStep = 3
	secondsPerMin = 60
	minsPerHour   = 60
)

var sizeUnits = []string{"KB", "MB", "GB", "TB"}

func formatSize(n int64) string {
	if n < byteUnit {
		return fmt.Sprintf("%d bytes", n)
	}
	value, unit := float64(n), -1
	for value >= byteUnit && unit < len(sizeUnits)-1 {
		value /= byteUnit
		unit++
	}
	return fmt.Sprintf("%.1f %s", value, sizeUnits[unit])
}

func formatCount(n int64) string {
	digits := strconv.FormatInt(n, 10)
	var sb strings.Builder
	for i, d := range digits {
		if i > 0 && (len(digits)-i)%thousandsStep == 0 {
			sb.WriteByte(',')
		}
		sb.WriteRune(d)
	}
	return sb.String()
}

func formatClock(d time.Duration) string {
	total := int(max(d, 0) / time.Second)
	hours, mins, secs := total/(secondsPerMin*minsPerHour), total/secondsPerMin%minsPerHour, total%secondsPerMin
	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d", hours, mins, secs)
	}
	return fmt.Sprintf("%d:%02d", mins, secs)
}
