//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"

	"github.com/ZerGo0/editor/internal/fsutil"
)

const (
	stressFixtureVersion          = 2
	stressCacheDirName            = "editor-e2e-stress"
	stressVersionPrefix           = "v"
	stressReadySuffix             = ".ready"
	stressBuildSuffix             = ".building"
	stressFixtureDiskBudget int64 = 3 << 30
	stressMinFreeDisk       int64 = 768 << 20
	statBlockBytes                = 512
	stressWriteWorkers            = 8
)

type stressFailure string

type stressT struct{}

func (stressT) Helper() {}

func (stressT) Fatal(args ...any) { panic(stressFailure(fmt.Sprint(args...))) }

func (stressT) Fatalf(format string, args ...any) { panic(stressFailure(fmt.Sprintf(format, args...))) }

type stressSpec struct {
	estimate int64
	build    func(t stressT, dir string)
}

var stressSizes sync.Map

func stressCacheRoot() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, stressCacheDirName, fmt.Sprintf("%s%d", stressVersionPrefix, stressFixtureVersion)), nil
}

func stressRoot(name string) string {
	root, _ := stressCacheRoot()
	return filepath.Join(root, name)
}

func stressFixtureBytes(name string) int64 {
	if size, ok := stressSizes.Load(name); ok {
		return size.(int64)
	}
	return 0
}

func allocatedBytes(root string) int64 {
	var total int64
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if info, err := d.Info(); err == nil {
			if st, ok := info.Sys().(*syscall.Stat_t); ok {
				total += st.Blocks * statBlockBytes
			} else {
				total += info.Size()
			}
		}
		return nil
	})
	return total
}

func removeOldFixtureVersions(root string) {
	parent := filepath.Dir(root)
	entries, _ := os.ReadDir(parent)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), stressVersionPrefix) && filepath.Join(parent, e.Name()) != root {
			_ = os.RemoveAll(filepath.Join(parent, e.Name()))
		}
	}
}

func buildStressFixture(root, name string, spec stressSpec) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if failure, ok := r.(stressFailure); ok {
				err = fmt.Errorf("fixture %s: %s", name, string(failure))
				return
			}
			panic(r)
		}
	}()
	building, dir := filepath.Join(root, name+stressBuildSuffix), filepath.Join(root, name)
	_ = os.RemoveAll(building)
	_ = os.RemoveAll(dir)
	if err := os.MkdirAll(building, 0o755); err != nil {
		return err
	}
	spec.build(stressT{}, building)
	size := allocatedBytes(building)
	if size > spec.estimate*2 {
		_ = os.RemoveAll(building)
		return fmt.Errorf("fixture %s took %d MiB, more than twice its %d MiB estimate", name, size>>20, spec.estimate>>20)
	}
	if err := os.Rename(building, dir); err != nil {
		return err
	}
	data, _ := json.Marshal(size)
	if err := os.WriteFile(filepath.Join(root, name+stressReadySuffix), data, 0o644); err != nil {
		return err
	}
	stressSizes.Store(name, size)
	return nil
}

func stressPrepare(t *testing.T) {
	t.Helper()
	root, err := stressCacheRoot()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	var missing []string
	var used, need int64
	for name, spec := range stressSpecs {
		if data, err := os.ReadFile(filepath.Join(root, name+stressReadySuffix)); err == nil {
			var size int64
			if json.Unmarshal(data, &size) == nil {
				stressSizes.Store(name, size)
				used += size
				continue
			}
		}
		missing = append(missing, name)
		need += spec.estimate
	}
	if len(missing) == 0 {
		return
	}
	removeOldFixtureVersions(root)
	if used+need > stressFixtureDiskBudget {
		t.Fatalf("fixtures %v need about %d MiB; %d MiB are used of the %d MiB budget", missing, need>>20, used>>20, stressFixtureDiskBudget>>20)
	}
	free, err := fsutil.FreeBytes(root)
	if err != nil {
		t.Fatal(err)
	}
	if int64(free)-need < stressMinFreeDisk {
		t.Fatalf("fixtures %v need about %d MiB; only %d MiB are free and %d MiB must stay free", missing, need>>20, free>>20, stressMinFreeDisk>>20)
	}
	errs := make([]error, len(missing))
	var wg sync.WaitGroup
	for i, name := range missing {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = buildStressFixture(root, name, stressSpecs[name])
		}()
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Error(err)
		}
	}
	if t.Failed() {
		t.FailNow()
	}
}
