package profiling

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"slices"
	"syscall"
	"time"

	"github.com/ZerGo0/editor/internal/applog"
)

const (
	DirName        = "profiles"
	GoroutinesFile = "goroutines.txt"
	HeapFile       = "heap.pb.gz"
	CPUFile        = "cpu.pb.gz"
	CPUDuration    = 30 * time.Second
	Keep           = 5
	stampLayout    = "20060102-150405.000"
	goroutineDebug = 2
	dirPerm        = 0o700
	partialSuffix  = ".partial"
)

func Listen(logsDir string) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGUSR1)
	applog.Go(func() {
		for range ch {
			log.Printf("profile: capturing for %s", CPUDuration)
			dir, err := Capture(filepath.Join(logsDir, DirName), time.Now(), CPUDuration)
			if err != nil {
				log.Printf("profile: %v", err)
				continue
			}
			log.Printf("profile: written to %s", dir)
		}
	})
}

func Capture(root string, now time.Time, cpu time.Duration) (string, error) {
	dir := filepath.Join(root, now.Format(stampLayout))
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return "", fmt.Errorf("profile dir: %w", err)
	}
	if err := writeFile(filepath.Join(dir, GoroutinesFile), func(f io.Writer) error {
		return pprof.Lookup("goroutine").WriteTo(f, goroutineDebug)
	}); err != nil {
		return "", err
	}
	runtime.GC()
	if err := writeFile(filepath.Join(dir, HeapFile), pprof.WriteHeapProfile); err != nil {
		return "", err
	}
	if err := writeFile(filepath.Join(dir, CPUFile), func(f io.Writer) error {
		if err := pprof.StartCPUProfile(f); err != nil {
			return err
		}
		time.Sleep(cpu)
		pprof.StopCPUProfile()
		return nil
	}); err != nil {
		return "", err
	}
	return dir, prune(root, Keep)
}

func writeFile(path string, write func(io.Writer) error) error {
	partial := path + partialSuffix
	f, err := os.Create(partial)
	if err != nil {
		return fmt.Errorf("profile file: %w", err)
	}
	if err := write(f); err != nil {
		f.Close()
		os.Remove(partial)
		return fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close %s: %w", filepath.Base(path), err)
	}
	return os.Rename(partial, path)
}

func prune(root string, keep int) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return fmt.Errorf("list profiles: %w", err)
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	slices.Sort(dirs)
	for _, name := range dirs[:max(0, len(dirs)-keep)] {
		if err := os.RemoveAll(filepath.Join(root, name)); err != nil {
			return fmt.Errorf("remove old profile: %w", err)
		}
	}
	return nil
}
