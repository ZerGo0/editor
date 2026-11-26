package viewer

import "math"

const (
	minScale      = 0.02
	maxScale      = 64
	wheelRate     = 0.01
	stopTolerance = 1.001
	percentFactor = 100
)

var zoomStops = []float32{0.05, 0.1, 0.125, 0.25, 0.5, 0.75, 1, 1.5, 2, 3, 4, 6, 8, 12, 16, 24, 32, maxScale}

type zoomState struct {
	fit          bool
	scale        float32
	panX, panY   float32
	enlargeToFit bool
}

func fitScale(viewW, viewH, imgW, imgH float32, enlarge bool) float32 {
	if viewW <= 0 || viewH <= 0 || imgW <= 0 || imgH <= 0 {
		return 1
	}
	s := min(viewW/imgW, viewH/imgH)
	if !enlarge {
		s = min(s, 1)
	}
	return clampScale(s)
}

func clampScale(s float32) float32 { return min(max(s, minScale), maxScale) }

func stepStop(scale float32, dir int) float32 {
	if dir > 0 {
		for _, stop := range zoomStops {
			if stop > scale*stopTolerance {
				return stop
			}
		}
		return maxScale
	}
	for i := len(zoomStops) - 1; i >= 0; i-- {
		if zoomStops[i] < scale/stopTolerance {
			return max(zoomStops[i], minScale)
		}
	}
	return minScale
}

func clampPan(pan, content, view float32) float32 {
	limit := max(0, (content-view)/2)
	return min(max(pan, -limit), limit)
}

func zoomAround(pan, oldScale, newScale, pointer, center float32) float32 {
	return pointer - center - (pointer-center-pan)*newScale/oldScale
}

func wheelFactor(dy float32) float32 { return float32(math.Exp(float64(-dy) * wheelRate)) }

func percentLabel(scale float32) int { return int(math.Round(float64(scale) * percentFactor)) }
