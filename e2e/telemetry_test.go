//go:build e2e

package e2e

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ZerGo0/editor/internal/telemetry"
)

const (
	e2eLogsDir     = "logs"
	maxTelemetryLn = 1 << 24
)

func TestTelemetryFilesAreJSONLines(t *testing.T) {
	t.Parallel()
	app, win, _ := openProject(t, map[string]string{"a.txt": "alpha\n"})
	app.Click(win, app.WaitNode(win, "a.txt row", RoleLabel("treeItem", "a.txt")), 2)
	app.WaitNode(win, "a.txt tab", notModified("a.txt"))
	app.Quit()
	dir := filepath.Join(app.UserData, e2eLogsDir, telemetry.DirName)
	if lines := jsonLines(t, filepath.Join(dir, telemetry.MetricsFile)); lines == 0 {
		t.Errorf("%s has no records", telemetry.MetricsFile)
	}
	if _, err := os.Stat(filepath.Join(dir, telemetry.TracesFile)); err == nil {
		jsonLines(t, filepath.Join(dir, telemetry.TracesFile))
	}
}

func jsonLines(t *testing.T, path string) int {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Errorf("telemetry file: %v", err)
		return 0
	}
	defer f.Close()
	n := 0
	sc := bufio.NewScanner(f)
	sc.Buffer(nil, maxTelemetryLn)
	for sc.Scan() {
		if !json.Valid(sc.Bytes()) {
			t.Errorf("%s line %d is not JSON: %.80s", filepath.Base(path), n+1, sc.Text())
		}
		n++
	}
	if err := sc.Err(); err != nil {
		t.Errorf("read %s: %v", path, err)
	}
	return n
}
