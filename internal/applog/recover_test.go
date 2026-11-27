package applog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func useCrashLog(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), crashFileName)
	setCrashPath(path)
	t.Cleanup(func() {
		setCrashPath("")
		OnRecovered(nil)
	})
	return path
}

func checkCrashLog(t *testing.T, path string, want ...string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range want {
		if !strings.Contains(string(data), w) {
			t.Errorf("crash log lacks %q:\n%s", w, data)
		}
	}
}

func TestRecoverWritesThePanicToTheCrashLog(t *testing.T) {
	path := useCrashLog(t)
	var got *Panic
	func() {
		defer Recover("test frame", func(p *Panic) { got = p })
		panic("boom")
	}()
	if got == nil || got.Where != "test frame" || got.Value != "boom" {
		t.Fatalf("handled %+v, want the boom panic of the test frame", got)
	}
	checkCrashLog(t, path, "recovered test frame: panic: boom", "TestRecoverWritesThePanicToTheCrashLog")
}

func TestGoRecoversAPanicAndTellsTheHook(t *testing.T) {
	path := useCrashLog(t)
	got := make(chan *Panic, 1)
	OnRecovered(func(p *Panic) { got <- p })
	Go(func() { panic("boom") })
	if p := <-got; p.Where != "goroutine" || p.Value != "boom" {
		t.Fatalf("hook got %+v, want the goroutine's boom panic", p)
	}
	checkCrashLog(t, path, "recovered goroutine: panic: boom")
}
