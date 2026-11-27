package qa

/*
#include <libproc.h>
#include <mach/mach_time.h>
#include <sys/resource.h>

int proc_rlimit_control(pid_t pid, int flavor, void *arg);

static int reset_footprint_interval(pid_t pid) {
	return proc_rlimit_control(pid, RLIMIT_FOOTPRINT_INTERVAL, (void *)(uintptr_t)FOOTPRINT_INTERVAL_RESET);
}
*/
import "C"

import (
	"fmt"
	"time"
	"unsafe"
)

func machTicks(ticks uint64) time.Duration {
	var base C.mach_timebase_info_data_t
	C.mach_timebase_info(&base)
	return time.Duration(ticks * uint64(base.numer) / uint64(base.denom))
}

func ReadUsage(pid int) (Usage, error) {
	var info C.struct_rusage_info_v4
	if rc := C.proc_pid_rusage(C.int(pid), C.RUSAGE_INFO_V4, (*C.rusage_info_t)(unsafe.Pointer(&info))); rc != 0 {
		return Usage{}, fmt.Errorf("proc_pid_rusage %d: code %d", pid, int(rc))
	}
	return Usage{
		CPU:          machTicks(uint64(info.ri_user_time) + uint64(info.ri_system_time)),
		ChildCPU:     machTicks(uint64(info.ri_child_user_time) + uint64(info.ri_child_system_time)),
		Footprint:    uint64(info.ri_phys_footprint),
		IntervalPeak: uint64(info.ri_interval_max_phys_footprint),
		LifetimePeak: uint64(info.ri_lifetime_max_phys_footprint),
		Resident:     uint64(info.ri_resident_size),
		DiskRead:     uint64(info.ri_diskio_bytesread),
		DiskWritten:  uint64(info.ri_diskio_byteswritten),
	}, nil
}

func ResetPeak(pid int) error {
	if rc := C.reset_footprint_interval(C.pid_t(pid)); rc != 0 {
		return fmt.Errorf("reset footprint interval %d: code %d", pid, int(rc))
	}
	return nil
}
