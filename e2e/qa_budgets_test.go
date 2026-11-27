//go:build e2e

package e2e

import (
	"time"

	"github.com/ZerGo0/editor/e2e/qa"
)

const (
	ms  = time.Millisecond
	kiB = 1 << 10
	miB = 1 << 20
)

type actionBudget struct {
	latency     time.Duration
	firstFrame  time.Duration
	cpu         time.Duration
	peak        uint64
	read        uint64
	write       uint64
	idle        float64
	data        int64
	startupCPU  time.Duration
	startupPeak uint64
	leak        uint64
}

func (b actionBudget) resources() qa.ResourceBudget {
	return qa.ResourceBudget{CPUPerRun: b.cpu, PeakFootprint: b.peak, DiskReadPerRun: b.read, DiskWrittenPerRun: b.write, IdleShare: b.idle, DataBytes: b.data}
}

func (b actionBudget) startup() qa.StartupBudget {
	return qa.StartupBudget{CPU: b.startupCPU, PeakFootprint: b.startupPeak}
}

var actionBudgets = map[string]actionBudget{
	"leak check: open and close 50 tabs": {
		latency: 120 * ms, firstFrame: 75 * ms, cpu: 45 * ms, peak: 199 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.025, data: 128 * kiB,
		startupCPU: 6300 * ms, startupPeak: 197 * miB,
		leak: 32 * miB,
	},
	"leak check: open the stacked diff 5 times": {
		latency: 1065 * ms, firstFrame: 45 * ms, cpu: 1885 * ms, peak: 244 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.025, data: 64 * kiB,
		startupCPU: 1800 * ms, startupPeak: 170 * miB,
		leak: 32 * miB,
	},
	"open the 100k-file folder": {
		latency: 4305 * ms, firstFrame: 4305 * ms, cpu: 6070 * ms, peak: 193 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.025, data: 64 * kiB,
		startupCPU: 6100 * ms, startupPeak: 193 * miB,
	},
	"expand a 20k-entry folder": {
		latency: 55 * ms, firstFrame: 30 * ms, cpu: 15 * ms, peak: 198 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.025, data: 64 * kiB,
		startupCPU: 6800 * ms, startupPeak: 192 * miB,
	},
	"type a query in Go to File over the big folder": {
		latency: 50 * ms, firstFrame: 35 * ms, cpu: 10 * ms, peak: 181 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.025, data: 64 * kiB,
		startupCPU: 6750 * ms, startupPeak: 181 * miB,
	},
	"text search over all files": {
		latency: 50 * ms, firstFrame: 30 * ms, cpu: 10 * ms, peak: 194 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.025, data: 64 * kiB,
		startupCPU: 6600 * ms, startupPeak: 192 * miB,
	},
	"switch tab with 200 tabs open": {
		latency: 135 * ms, firstFrame: 55 * ms, cpu: 55 * ms, peak: 209 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.025, data: 64 * kiB,
		startupCPU: 6750 * ms, startupPeak: 208 * miB,
	},
	"close 200 tabs": {
		latency: 50 * ms, firstFrame: 30 * ms, cpu: 10 * ms, peak: 191 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.025, data: 64 * kiB,
		startupCPU: 6600 * ms, startupPeak: 191 * miB,
	},
	"open the Graph (200k commits)": {
		latency: 100 * ms, firstFrame: 100 * ms, cpu: 495 * ms, peak: 150 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.025, data: 64 * kiB,
		startupCPU: 700 * ms, startupPeak: 145 * miB,
	},
	"scroll the Graph (200k commits)": {
		latency: 670 * ms, firstFrame: 30 * ms, cpu: 480 * ms, peak: 153 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.025, data: 128 * kiB,
		startupCPU: 750 * ms, startupPeak: 147 * miB,
	},
	"open Source Control (200k commits)": {
		latency: 425 * ms, firstFrame: 55 * ms, cpu: 450 * ms, peak: 150 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.025, data: 64 * kiB,
		startupCPU: 650 * ms, startupPeak: 146 * miB,
	},
	"open Source Control with 10k changes": {
		latency: 150 * ms, firstFrame: 80 * ms, cpu: 100 * ms, peak: 182 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.025, data: 64 * kiB,
		startupCPU: 1850 * ms, startupPeak: 168 * miB,
	},
	"open the stacked Staged Changes diff (10k files)": {
		latency: 50 * ms, firstFrame: 50 * ms, cpu: 2420 * ms, peak: 221 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.025, data: 64 * kiB,
		startupCPU: 1850 * ms, startupPeak: 167 * miB,
	},
	"open the 1 GB file": {
		latency: 700 * ms, firstFrame: 75 * ms, cpu: 225 * ms, peak: 161 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.025, data: 64 * kiB,
		startupCPU: 500 * ms, startupPeak: 139 * miB,
	},
	"type a character in the 1 GB file": {
		latency: 50 * ms, firstFrame: 30 * ms, cpu: 10 * ms, peak: 159 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.025, data: 64 * kiB,
		startupCPU: 950 * ms, startupPeak: 158 * miB,
	},
	"scroll to the end of the 1 GB file": {
		latency: 50 * ms, firstFrame: 30 * ms, cpu: 20 * ms, peak: 159 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.025, data: 64 * kiB,
		startupCPU: 900 * ms, startupPeak: 158 * miB,
	},
	"search in the 1 GB file": {
		latency: 50 * ms, firstFrame: 30 * ms, cpu: 20 * ms, peak: 157 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.025, data: 64 * kiB,
		startupCPU: 900 * ms, startupPeak: 157 * miB,
	},
	"open the 50 MB single-line file": {
		latency: 95 * ms, firstFrame: 95 * ms, cpu: 55 * ms, peak: 158 * miB,
		read: 64 * miB, write: 64 * kiB, idle: 0.025, data: 64 * kiB,
		startupCPU: 550 * ms, startupPeak: 141 * miB,
	},
	"type a character in the 50 MB single-line file": {
		latency: 50 * ms, firstFrame: 30 * ms, cpu: 10 * ms, peak: 165 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.025, data: 64 * kiB,
		startupCPU: 600 * ms, startupPeak: 161 * miB,
	},
	"scroll to the end of the 50 MB single-line file": {
		latency: 50 * ms, firstFrame: 30 * ms, cpu: 15 * ms, peak: 152 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.025, data: 64 * kiB,
		startupCPU: 500 * ms, startupPeak: 145 * miB,
	},
	"search in the 50 MB single-line file": {
		latency: 50 * ms, firstFrame: 30 * ms, cpu: 20 * ms, peak: 159 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.025, data: 64 * kiB,
		startupCPU: 600 * ms, startupPeak: 152 * miB,
	},
	"open the 200k-line file": {
		latency: 110 * ms, firstFrame: 85 * ms, cpu: 65 * ms, peak: 186 * miB,
		read: 16 * miB, write: 64 * kiB, idle: 0.025, data: 64 * kiB,
		startupCPU: 600 * ms, startupPeak: 139 * miB,
	},
	"type a character in the 200k-line file": {
		latency: 50 * ms, firstFrame: 30 * ms, cpu: 15 * ms, peak: 174 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.036000000000000004, data: 9 * miB,
		startupCPU: 600 * ms, startupPeak: 172 * miB,
	},
	"scroll to the end of the 200k-line file": {
		latency: 50 * ms, firstFrame: 30 * ms, cpu: 50 * ms, peak: 175 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.025, data: 64 * kiB,
		startupCPU: 650 * ms, startupPeak: 165 * miB,
	},
	"search in the 200k-line file": {
		latency: 65 * ms, firstFrame: 50 * ms, cpu: 60 * ms, peak: 216 * miB,
		read: 64 * kiB, write: 3 * miB, idle: 0.036000000000000004, data: 9 * miB,
		startupCPU: 450 * ms, startupPeak: 170 * miB,
	},
	"type a character in a 5k-line file": {
		latency: 255 * ms, firstFrame: 215 * ms, cpu: 50 * ms, peak: 180 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.025, data: 384 * kiB,
		startupCPU: 1600 * ms, startupPeak: 165 * miB,
	},
	"scroll the 5k-line file": {
		latency: 120 * ms, firstFrame: 120 * ms, cpu: 15 * ms, peak: 170 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.025, data: 64 * kiB,
		startupCPU: 1650 * ms, startupPeak: 166 * miB,
	},
	"open Go to File": {
		latency: 95 * ms, firstFrame: 60 * ms, cpu: 20 * ms, peak: 162 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.025, data: 64 * kiB,
		startupCPU: 1650 * ms, startupPeak: 157 * miB,
	},
	"type a query in Go to File": {
		latency: 50 * ms, firstFrame: 45 * ms, cpu: 20 * ms, peak: 159 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.025, data: 64 * kiB,
		startupCPU: 1600 * ms, startupPeak: 158 * miB,
	},
	"switch tab": {
		latency: 120 * ms, firstFrame: 65 * ms, cpu: 45 * ms, peak: 163 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.025, data: 64 * kiB,
		startupCPU: 1300 * ms, startupPeak: 156 * miB,
	},
	"toggle sidebar": {
		latency: 50 * ms, firstFrame: 30 * ms, cpu: 15 * ms, peak: 160 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.025, data: 64 * kiB,
		startupCPU: 1500 * ms, startupPeak: 156 * miB,
	},
	"expand a 2000-entry folder": {
		latency: 50 * ms, firstFrame: 35 * ms, cpu: 25 * ms, peak: 162 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.025, data: 64 * kiB,
		startupCPU: 1400 * ms, startupPeak: 156 * miB,
	},
	"open Source Control with 500+ changes": {
		latency: 140 * ms, firstFrame: 75 * ms, cpu: 55 * ms, peak: 174 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.025, data: 64 * kiB,
		startupCPU: 1550 * ms, startupPeak: 156 * miB,
	},
	"open the stacked Staged Changes diff (210 files)": {
		latency: 200 * ms, firstFrame: 50 * ms, cpu: 230 * ms, peak: 190 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.025, data: 64 * kiB,
		startupCPU: 1300 * ms, startupPeak: 157 * miB,
	},
	"open the Graph (2100 commits)": {
		latency: 105 * ms, firstFrame: 90 * ms, cpu: 50 * ms, peak: 161 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.025, data: 64 * kiB,
		startupCPU: 1600 * ms, startupPeak: 156 * miB,
	},
	"save the 5k-line file": {
		latency: 1315 * ms, firstFrame: 50 * ms, cpu: 435 * ms, peak: 179 * miB,
		read: 64 * kiB, write: 448 * kiB, idle: 0.025, data: 320 * kiB,
		startupCPU: 1550 * ms, startupPeak: 164 * miB,
	},
}
