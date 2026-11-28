//go:build e2e

package e2e

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ZerGo0/editor/e2e/qa"
	"github.com/ZerGo0/editor/internal/backup"
)

const (
	stressDirtyFiles    = 100
	stressDirtyUntitled = 100
	hotExitBackupsDir   = "backups"
	hotExitProjectBytes = 8
	untitledPathPrefix  = "untitled:Untitled-"
	untitledDraftLines  = 40
	dirtyFileSuffix     = "// unsaved edit\n"
)

func stressBackupProjectKey(root string) string {
	sum := sha256.Sum256([]byte(root))
	return hex.EncodeToString(sum[:hotExitProjectBytes])
}

func stressUntitledText(i int) string {
	var b strings.Builder
	for line := range untitledDraftLines {
		fmt.Fprintf(&b, "draft %d line %d: notes that were never saved\n", i, line)
	}
	return b.String()
}

func seedStressBackups(t *testing.T, root, userData string) {
	t.Helper()
	store := backup.Open(filepath.Join(userData, historyDataDir, hotExitBackupsDir, stressBackupProjectKey(root)), backup.DefaultLimit)
	var seq uint64
	write := func(meta backup.Meta, text string) {
		seq++
		meta.Size = int64(len(text))
		if err := store.Write(seq, meta, strings.NewReader(text), meta.Size); err != nil {
			t.Fatal(err)
		}
	}
	for i := range stressDirtyFiles {
		path := stressTreeTabFile(root, i)
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		write(backup.Meta{Path: path, DiskSize: info.Size(), DiskModTime: info.ModTime()}, string(data)+dirtyFileSuffix)
	}
	for i := 1; i <= stressDirtyUntitled; i++ {
		write(backup.Meta{Path: fmt.Sprintf("%s%d", untitledPathPrefix, i)}, stressUntitledText(i))
	}
	if got := len(store.Entries()); got != stressDirtyFiles+stressDirtyUntitled {
		t.Fatalf("seeded %d backups, want %d", got, stressDirtyFiles+stressDirtyUntitled)
	}
}

func stressHotExitOptions(d DemoProject) QAOptions {
	o := stressOptions(nil)(d)
	o.Seed = seedStressBackups
	return o
}

func expectRestoredBuffers(q *QAApp, d DemoProject) error {
	last := fmt.Sprintf("Untitled-%d", stressDirtyUntitled)
	if _, err := q.TryWait(RoleLabel("tab", last), q.waitFor()); err != nil {
		return fmt.Errorf("the last restored buffer %s has no tab: %w", last, err)
	}
	return nil
}

func hotExitRestoreAction() latencyAction {
	return latencyAction{Owner: qa.OwnerWorkspace, IdleFrames: blinkFramesPerSec, Verify: expectRestoredBuffers}
}

func stressHotExitActions() []latencyAction {
	name := fmt.Sprintf("restart and restore %d unsaved buffers", stressDirtyFiles+stressDirtyUntitled)
	return []latencyAction{
		hotExitRestoreAction().onFixture(stressFixtureTree, name, stressQuietLong, 0).withOptions(stressHotExitOptions),
	}
}
