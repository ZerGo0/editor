package qa

import (
	"fmt"
	"time"
)

const (
	RuleCPU      = "cpu-budget"
	RuleMemory   = "memory-budget"
	RuleDiskIO   = "disk-io-budget"
	RuleIdleCPU  = "idle-cpu"
	RuleDataSize = "data-size-budget"
	RuleStartup  = "startup-budget"
	RuleLeak     = "memory-leak"
	bytesPerKiB  = 1 << 10
	bytesPerMiB  = 1 << 20
)

type Usage struct {
	CPU          time.Duration
	ChildCPU     time.Duration
	Footprint    uint64
	IntervalPeak uint64
	LifetimePeak uint64
	Resident     uint64
	DiskRead     uint64
	DiskWritten  uint64
}

type Work struct {
	Runs           int
	CPU            time.Duration
	ChildCPU       time.Duration
	StartFootprint uint64
	PeakFootprint  uint64
	EndResident    uint64
	DiskRead       uint64
	DiskWritten    uint64
}

func Between(start, end Usage, runs int) Work {
	return Work{
		Runs:           runs,
		CPU:            end.CPU - start.CPU,
		ChildCPU:       end.ChildCPU - start.ChildCPU,
		StartFootprint: start.Footprint,
		PeakFootprint:  end.IntervalPeak,
		EndResident:    end.Resident,
		DiskRead:       end.DiskRead - start.DiskRead,
		DiskWritten:    end.DiskWritten - start.DiskWritten,
	}
}

func (w Work) CPUPerRun() time.Duration {
	return (w.CPU + w.ChildCPU) / time.Duration(max(w.Runs, 1))
}

func (w Work) perRun(bytes uint64) uint64 { return bytes / uint64(max(w.Runs, 1)) }

type Idle struct {
	CPU    time.Duration
	Window time.Duration
}

func (i Idle) Share() float64 {
	if i.Window <= 0 {
		return 0
	}
	return float64(i.CPU) / float64(i.Window)
}

type ResourceBudget struct {
	CPUPerRun         time.Duration
	PeakFootprint     uint64
	DiskReadPerRun    uint64
	DiskWrittenPerRun uint64
	IdleShare         float64
	DataBytes         int64
}

func MiB(bytes uint64) string { return fmt.Sprintf("%.1f MiB", float64(bytes)/bytesPerMiB) }

func KiB(bytes uint64) string { return fmt.Sprintf("%.1f KiB", float64(bytes)/bytesPerKiB) }

func resourceFlag(rule, action, owner, measured, expected, fix string) Flag {
	return Flag{Rule: rule, View: action, Owner: owner, Role: "action", Label: action, Measured: measured, Expected: expected, Fix: fix}
}

type StartupBudget struct {
	CPU           time.Duration
	PeakFootprint uint64
}

func OverStartup(owner string, u Usage, b StartupBudget) []Flag {
	const view = "app startup"
	var flags []Flag
	if cpu := u.CPU + u.ChildCPU; cpu > b.CPU {
		flags = append(flags, resourceFlag(RuleStartup, view, owner,
			fmt.Sprintf("%s CPU until the app is quiet (app %s, children %s)", cpu.Round(time.Millisecond), u.CPU.Round(time.Millisecond), u.ChildCPU.Round(time.Millisecond)),
			"at most "+b.CPU.String()+" CPU for startup",
			"profile the launch (SIGUSR1 pprof) and defer or cache what the first window does not need"))
	}
	if u.LifetimePeak > b.PeakFootprint {
		flags = append(flags, resourceFlag(RuleStartup, view, owner,
			"peak footprint "+MiB(u.LifetimePeak)+" at startup",
			"at most "+MiB(b.PeakFootprint),
			"find what the launch keeps in memory (heap profile) and load it lazily"))
	}
	return flags
}

func OverGrowth(action, owner string, growth int64, budget uint64) []Flag {
	if budget == 0 || growth <= int64(budget) {
		return nil
	}
	return []Flag{resourceFlag(RuleLeak, action, owner,
		"footprint grew "+MiB(uint64(growth))+" after the work and a quiet wait",
		"at most "+MiB(budget)+" above the baseline",
		"find what the closed tabs or views keep alive (heap profile) and release it")}
}

func OverResources(action, owner string, w Work, idle Idle, dataBytes int64, b ResourceBudget) []Flag {
	var flags []Flag
	if cpu := w.CPUPerRun(); cpu > b.CPUPerRun {
		flags = append(flags, resourceFlag(RuleCPU, action, owner,
			fmt.Sprintf("%s CPU per run (app %s, children %s over %d runs)", cpu.Round(time.Millisecond/10), w.CPU.Round(time.Millisecond), w.ChildCPU.Round(time.Millisecond), w.Runs),
			"at most "+b.CPUPerRun.String()+" CPU per run",
			"profile the run with pprof (SIGUSR1) and remove repeated work, or move it off the UI thread"))
	}
	if w.PeakFootprint > b.PeakFootprint {
		flags = append(flags, resourceFlag(RuleMemory, action, owner,
			"peak footprint "+MiB(w.PeakFootprint)+" (start "+MiB(w.StartFootprint)+")",
			"at most "+MiB(b.PeakFootprint),
			"find what the action keeps in memory (heap profile) and stream, virtualize or release it"))
	}
	if read, written := w.perRun(w.DiskRead), w.perRun(w.DiskWritten); read > b.DiskReadPerRun || written > b.DiskWrittenPerRun {
		flags = append(flags, resourceFlag(RuleDiskIO, action, owner,
			fmt.Sprintf("read %s, wrote %s per run", KiB(read), KiB(written)),
			fmt.Sprintf("read at most %s, write at most %s per run", KiB(b.DiskReadPerRun), KiB(b.DiskWrittenPerRun)),
			"find the repeated reads or writes (fs_usage) and cache or batch them"))
	}
	if idle.Share() > b.IdleShare {
		flags = append(flags, resourceFlag(RuleIdleCPU, action, owner,
			fmt.Sprintf("%s CPU in %s of quiet (%.2f%% of a core)", idle.CPU.Round(time.Millisecond/10), idle.Window.Round(time.Millisecond), idle.Share()*percent),
			fmt.Sprintf("at most %.2f%% of a core while idle", b.IdleShare*percent),
			"find the timer, watcher or After loop that keeps working while the view is static"))
	}
	if dataBytes > b.DataBytes {
		flags = append(flags, resourceFlag(RuleDataSize, action, owner,
			KiB(uint64(dataBytes))+" of app data on disk",
			"at most "+KiB(uint64(b.DataBytes)),
			"find which store (history, backups, shelf, logs, temp) grew and bound or clean it"))
	}
	return flags
}

const percent = 100
