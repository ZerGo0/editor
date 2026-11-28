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
		latency: 490 * ms, firstFrame: 35 * ms, cpu: 70 * ms, peak: 181 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.039, data: 64 * kiB,
		startupCPU: 6750 * ms, startupPeak: 181 * miB,
	},
	"text search over all files": {
		latency: 1115 * ms, firstFrame: 30 * ms, cpu: 3210 * ms, peak: 194 * miB,
		read: 53 * miB, write: 64 * kiB, idle: 0.041, data: 256 * kiB,
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
		startupCPU: 1700 * ms, startupPeak: 167 * miB,
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
		startupCPU: 1150 * ms, startupPeak: 165 * miB,
	},
	"scroll the 5k-line file": {
		latency: 120 * ms, firstFrame: 120 * ms, cpu: 15 * ms, peak: 170 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.025, data: 64 * kiB,
		startupCPU: 1200 * ms, startupPeak: 166 * miB,
	},
	"scroll the pull request list (1000 PRs)": {
		latency: 50 * ms, firstFrame: 30 * ms, cpu: 30 * ms, peak: 320 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.025, data: 128 * kiB,
		startupCPU: 800 * ms, startupPeak: 280 * miB,
	},
	"open a pull request with 300 changed files": {
		latency: 75 * ms, firstFrame: 50 * ms, cpu: 120 * ms, peak: 330 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.025, data: 64 * kiB,
		startupCPU: 800 * ms, startupPeak: 280 * miB,
	},
	"open Go to File": {
		latency: 105 * ms, firstFrame: 80 * ms, cpu: 20 * ms, peak: 162 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.036, data: 64 * kiB,
		startupCPU: 1250 * ms, startupPeak: 157 * miB,
	},
	"type a query in Go to File": {
		latency: 65 * ms, firstFrame: 45 * ms, cpu: 30 * ms, peak: 159 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.043, data: 128 * kiB,
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
		startupCPU: 1150 * ms, startupPeak: 156 * miB,
	},
	"save the 5k-line file": {
		latency: 1315 * ms, firstFrame: 50 * ms, cpu: 435 * ms, peak: 179 * miB,
		read: 64 * kiB, write: 448 * kiB, idle: 0.025, data: 320 * kiB,
		startupCPU: 1550 * ms, startupPeak: 164 * miB,
	},
	"open 8 editor groups with 50 tabs each": {
		latency: 8070 * ms, firstFrame: 8070 * ms, cpu: 7040 * ms, peak: 369 * miB,
		read: 2 * miB, write: 192 * kiB, idle: 0.069, data: 192 * kiB,
		startupCPU: 7050 * ms, startupPeak: 369 * miB,
	},
	"split the editor into 8 groups": {
		latency: 85 * ms, firstFrame: 35 * ms, cpu: 30 * ms, peak: 346 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.05, data: 64 * kiB,
		startupCPU: 8200 * ms, startupPeak: 330 * miB,
	},
	"switch focus between 8 editor groups": {
		latency: 50 * ms, firstFrame: 45 * ms, cpu: 45 * ms, peak: 364 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.039, data: 192 * kiB,
		startupCPU: 8450 * ms, startupPeak: 360 * miB,
	},
	"close 8 editor groups with 50 tabs each": {
		latency: 50 * ms, firstFrame: 30 * ms, cpu: 70 * ms, peak: 356 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.041, data: 192 * kiB,
		startupCPU: 7600 * ms, startupPeak: 355 * miB,
	},
	"open the Command Palette": {
		latency: 75 * ms, firstFrame: 35 * ms, cpu: 25 * ms, peak: 321 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.045, data: 64 * kiB,
		startupCPU: 550 * ms, startupPeak: 265 * miB,
	},
	"type a query in the Command Palette": {
		latency: 75 * ms, firstFrame: 30 * ms, cpu: 30 * ms, peak: 318 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.025, data: 64 * kiB,
		startupCPU: 600 * ms, startupPeak: 265 * miB,
	},
	"scroll the Command Palette results": {
		latency: 130 * ms, firstFrame: 100 * ms, cpu: 30 * ms, peak: 319 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.059, data: 64 * kiB,
		startupCPU: 600 * ms, startupPeak: 264 * miB,
	},
	"open Search Everywhere over the big folder": {
		latency: 85 * ms, firstFrame: 65 * ms, cpu: 25 * ms, peak: 344 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.025, data: 64 * kiB,
		startupCPU: 7300 * ms, startupPeak: 322 * miB,
	},
	"type a query in Search Everywhere (All) over the big folder": {
		latency: 370 * ms, firstFrame: 40 * ms, cpu: 4485 * ms, peak: 381 * miB,
		read: 138 * miB, write: 64 * kiB, idle: 0.026, data: 640 * kiB,
		startupCPU: 6750 * ms, startupPeak: 320 * miB,
	},
	"type a query in Search Everywhere (Files) over the big folder": {
		latency: 315 * ms, firstFrame: 65 * ms, cpu: 70 * ms, peak: 358 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.025, data: 64 * kiB,
		startupCPU: 6700 * ms, startupPeak: 322 * miB,
	},
	"filter the Settings list": {
		latency: 55 * ms, firstFrame: 55 * ms, cpu: 20 * ms, peak: 316 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.063, data: 64 * kiB,
		startupCPU: 600 * ms, startupPeak: 264 * miB,
	},
	"scroll the Settings list": {
		latency: 170 * ms, firstFrame: 45 * ms, cpu: 25 * ms, peak: 317 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.029, data: 64 * kiB,
		startupCPU: 650 * ms, startupPeak: 264 * miB,
	},
	"filter the Keyboard Shortcuts list (2160 user bindings)": {
		latency: 65 * ms, firstFrame: 65 * ms, cpu: 60 * ms, peak: 331 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.026, data: 384 * kiB,
		startupCPU: 600 * ms, startupPeak: 267 * miB,
	},
	"scroll the Keyboard Shortcuts list (2160 user bindings)": {
		latency: 125 * ms, firstFrame: 45 * ms, cpu: 75 * ms, peak: 331 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.042, data: 320 * kiB,
		startupCPU: 600 * ms, startupPeak: 268 * miB,
	},
	"scroll the Search Everywhere (Files) results over the big folder": {
		latency: 140 * ms, firstFrame: 30 * ms, cpu: 20 * ms, peak: 378 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.028, data: 64 * kiB,
		startupCPU: 6400 * ms, startupPeak: 322 * miB,
	},
	"open the TODO panel (20k TODO comments)": {
		latency: 50 * ms, firstFrame: 35 * ms, cpu: 35 * ms, peak: 340 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.025, data: 64 * kiB,
		startupCPU: 1000 * ms, startupPeak: 277 * miB,
	},
	"scroll the TODO panel (20k TODO comments)": {
		latency: 55 * ms, firstFrame: 40 * ms, cpu: 25 * ms, peak: 333 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.032, data: 64 * kiB,
		startupCPU: 1100 * ms, startupPeak: 298 * miB,
	},
	"open the Problems panel (10k problems)": {
		latency: 55 * ms, firstFrame: 40 * ms, cpu: 60 * ms, peak: 430 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.025, data: 576 * kiB,
		startupCPU: 950 * ms, startupPeak: 312 * miB,
	},
	"filter the Problems panel (10k problems)": {
		latency: 50 * ms, firstFrame: 30 * ms, cpu: 35 * ms, peak: 411 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.033, data: 576 * kiB,
		startupCPU: 950 * ms, startupPeak: 310 * miB,
	},
	"open the history of a file (500 versions)": {
		latency: 55 * ms, firstFrame: 30 * ms, cpu: 90 * ms, peak: 325 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.025, data: 960 * kiB,
		startupCPU: 700 * ms, startupPeak: 289 * miB,
	},
	"select versions in the history of a file (500 versions)": {
		latency: 60 * ms, firstFrame: 30 * ms, cpu: 30 * ms, peak: 328 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.025, data: 960 * kiB,
		startupCPU: 650 * ms, startupPeak: 289 * miB,
	},
	"restart and restore 200 unsaved buffers": {
		latency: 10000 * ms, firstFrame: 10000 * ms, cpu: 5950 * ms, peak: 345 * miB,
		read: 2 * miB, write: 64 * kiB, idle: 0.055, data: 448 * kiB,
		startupCPU: 5950 * ms, startupPeak: 345 * miB,
	},
	"turn on word wrap in the 200k-line file": {
		latency: 50 * ms, firstFrame: 30 * ms, cpu: 35 * ms, peak: 354 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.029, data: 64 * kiB,
		startupCPU: 650 * ms, startupPeak: 311 * miB,
	},
	"scroll to the end of the 200k-line file with word wrap": {
		latency: 50 * ms, firstFrame: 30 * ms, cpu: 60 * ms, peak: 345 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.029, data: 64 * kiB,
		startupCPU: 700 * ms, startupPeak: 337 * miB,
	},
	"turn on word wrap in the 100k-line file with 160 characters per line": {
		latency: 50 * ms, firstFrame: 30 * ms, cpu: 380 * ms, peak: 395 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.052, data: 128 * kiB,
		startupCPU: 650 * ms, startupPeak: 347 * miB,
	},
	"scroll to the end of the 100k-line file with 160 characters per line with word wrap": {
		latency: 50 * ms, firstFrame: 30 * ms, cpu: 25 * ms, peak: 358 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.025, data: 64 * kiB,
		startupCPU: 650 * ms, startupPeak: 330 * miB,
	},
	"scroll the issue list (1000 issues)": {
		latency: 50 * ms, firstFrame: 30 * ms, cpu: 30 * ms, peak: 337 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.03, data: 128 * kiB,
		startupCPU: 900 * ms, startupPeak: 288 * miB,
	},
	"scroll the notification list (500 notifications)": {
		latency: 55 * ms, firstFrame: 55 * ms, cpu: 35 * ms, peak: 323 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.025, data: 64 * kiB,
		startupCPU: 900 * ms, startupPeak: 288 * miB,
	},
	"switch branch with 10k changed files (50k commits)": {
		latency: 14585 * ms, firstFrame: 30 * ms, cpu: 6485 * ms, peak: 338 * miB,
		read: 64 * kiB, write: 64 * kiB, idle: 0.04, data: 192 * kiB,
		startupCPU: 3650 * ms, startupPeak: 303 * miB,
	},
}
