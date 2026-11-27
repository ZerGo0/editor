//go:build e2e

package e2e

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

const (
	stressTreeLeafFiles     = 10
	stressTreeLeafDirs      = 8000
	stressFlatFiles         = 20000
	stressFlatDir           = "flat"
	stressHistoryCommits    = 200000
	stressHistoryFiles      = 20
	stressChangedFiles      = 10000
	stressChangedDirs       = 100
	stressHugeFileBytes     = 1000000000
	stressMinifiedBytes     = 50000000
	stressLongFileLines     = 200000
	stressHugeBlockLines    = 60000
	stressNeedle            = "stressneedle"
	stressEndNeedle         = "stress-needle-at-the-end"
	stressGitDate           = 1767225600
	stressCommitStepSeconds = 60
	stressFileBufferBytes   = 1 << 20
	stressHugeName          = "huge-1gb.txt"
	stressMinifiedName      = "minified-50mb.js"
	stressLongName          = "lines-200k.go"
	stressFixtureTree       = "tree"
	stressFixtureHistory    = "history"
	stressFixtureChanges    = "changes"
	stressFixtureFiles      = "files"
	stressTreeFiles         = stressTreeLeafDirs*stressTreeLeafFiles + stressFlatFiles
)

var stressSpecs = map[string]stressSpec{
	stressFixtureTree:    {estimate: 520 << 20, build: buildTreeFixture},
	stressFixtureHistory: {estimate: 220 << 20, build: buildHistoryFixture},
	stressFixtureChanges: {estimate: 140 << 20, build: buildChangesFixture},
	stressFixtureFiles:   {estimate: 1100 << 20, build: buildFilesFixture},
}

func stressGitEnv() []string {
	return append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0", "LC_ALL=C")
}

func stressGit(t stressT, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir, cmd.Env = dir, stressGitEnv()
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func stressFastImport(t stressT, dir string, write func(w io.Writer)) {
	t.Helper()
	cmd := exec.Command("git", "fast-import", "--quiet")
	cmd.Dir, cmd.Env = dir, stressGitEnv()
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	w := bufio.NewWriterSize(stdin, stressFileBufferBytes)
	write(w)
	if err := w.Flush(); err != nil {
		t.Fatalf("fast-import stream: %v\n%s", err, stderr.String())
	}
	_ = stdin.Close()
	if err := cmd.Wait(); err != nil {
		t.Fatalf("git fast-import: %v\n%s", err, stderr.String())
	}
}

func fastImportFile(w io.Writer, path, content string) {
	fmt.Fprintf(w, "M 100644 inline %s\ndata %d\n%s\n", path, len(content), content)
}

func fastImportCommit(w io.Writer, n int, message string) {
	fmt.Fprintf(w, "commit refs/heads/main\ncommitter Ada Lovelace <ada@example.invalid> %d +0000\ndata %d\n%s\n", stressGitDate+n*stressCommitStepSeconds, len(message), message)
}

func stressInitRepo(t stressT, dir string) {
	t.Helper()
	stressGit(t, dir, "init", "-q", "-b", "main")
}

func stressCheckout(t stressT, dir string) {
	t.Helper()
	stressGit(t, dir, "reset", "-q", "--hard", "main")
}

func stressTreePath(i int) string {
	leaf := i / stressTreeLeafFiles
	return fmt.Sprintf("pkg%02d/mod%02d/lib%02d/part%d/file%05d.go", leaf/800, leaf/80%10, leaf/8%10, leaf%8, i)
}

func stressFlatPath(i int) string { return fmt.Sprintf("%s/entry%05d.txt", stressFlatDir, i) }

func stressTreeContent(i int) string {
	return fmt.Sprintf("package part\n\n// stress file %d\nvar %s%d = \"%s\"\n", i, stressNeedle, i, stressNeedle)
}

func stressFlatContent(i int) string { return fmt.Sprintf("entry %d %s\n", i, stressNeedle) }

func stressChangedContent(i int) string {
	return fmt.Sprintf("package src\n\nfunc F%d() int { return %d }\n", i, i)
}

func stressChangedEdit(i int) string {
	return fmt.Sprintf("package src\n\n// changed\nfunc F%d() int { return %d + 1 }\n", i, i)
}

func writeFilesParallel(t stressT, dir string, count int, path func(i int) string, content func(i int) string) {
	t.Helper()
	var wg sync.WaitGroup
	errs := make([]error, stressWriteWorkers)
	for w := range stressWriteWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := w; i < count; i += stressWriteWorkers {
				full := filepath.Join(dir, filepath.FromSlash(path(i)))
				if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
					errs[w] = err
					return
				}
				if err := os.WriteFile(full, []byte(content(i)), 0o644); err != nil {
					errs[w] = err
					return
				}
			}
		}()
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
}

func stressImportAndIndex(t stressT, dir string, write func(w io.Writer)) {
	t.Helper()
	stressFastImport(t, dir, write)
	stressGit(t, dir, "reset", "-q", "main")
	stressGit(t, dir, "update-index", "-q", "--refresh")
}

func buildTreeFixture(t stressT, dir string) {
	stressInitRepo(t, dir)
	const treeFiles = stressTreeLeafDirs * stressTreeLeafFiles
	var wg sync.WaitGroup
	var failure any
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() { failure = recover() }()
		writeFilesParallel(t, dir, treeFiles, stressTreePath, stressTreeContent)
		writeFilesParallel(t, dir, stressFlatFiles, stressFlatPath, stressFlatContent)
	}()
	stressFastImport(t, dir, func(w io.Writer) {
		fastImportCommit(w, 0, "100k files")
		for i := range treeFiles {
			fastImportFile(w, stressTreePath(i), stressTreeContent(i))
		}
		for i := range stressFlatFiles {
			fastImportFile(w, stressFlatPath(i), stressFlatContent(i))
		}
	})
	wg.Wait()
	if failure != nil {
		panic(failure)
	}
	stressGit(t, dir, "reset", "-q", "main")
	stressGit(t, dir, "update-index", "-q", "--refresh")
}

func buildHistoryFixture(t stressT, dir string) {
	stressInitRepo(t, dir)
	stressFastImport(t, dir, func(w io.Writer) {
		for n := range stressHistoryCommits {
			file := n % stressHistoryFiles
			fastImportCommit(w, n, fmt.Sprintf("commit %d: update f%02d", n, file))
			fastImportFile(w, fmt.Sprintf("f%02d.txt", file), fmt.Sprintf("revision %d of file %d\n", n, file))
		}
	})
	stressCheckout(t, dir)
}

func stressChangedPath(i int) string {
	return fmt.Sprintf("src/d%02d/c%05d.go", i%stressChangedDirs, i)
}

func buildChangesFixture(t stressT, dir string) {
	stressInitRepo(t, dir)
	writeFilesParallel(t, dir, stressChangedFiles, stressChangedPath, stressChangedContent)
	stressImportAndIndex(t, dir, func(w io.Writer) {
		fastImportCommit(w, 0, "10k files")
		for i := range stressChangedFiles {
			fastImportFile(w, stressChangedPath(i), stressChangedContent(i))
		}
	})
	writeFilesParallel(t, dir, stressChangedFiles, stressChangedPath, stressChangedEdit)
	stressGit(t, dir, "add", "-u")
}

func buildFilesFixture(t stressT, dir string) {
	var block bytes.Buffer
	for i := range stressHugeBlockLines {
		fmt.Fprintf(&block, "line %06d: the quick brown fox jumps over the lazy dog 0123456789\n", i)
	}
	writeRepeated(t, filepath.Join(dir, stressHugeName), block.Bytes(), stressHugeFileBytes, stressEndNeedle+"\n")
	writeRepeated(t, filepath.Join(dir, stressMinifiedName), []byte("a=1,b=2;"), stressMinifiedBytes, stressEndNeedle)
	var lines bytes.Buffer
	for i := range stressLongFileLines {
		fmt.Fprintf(&lines, "func f%d() int { return %d }\n", i, i)
	}
	fmt.Fprintf(&lines, "// %s\n", stressEndNeedle)
	if err := os.WriteFile(filepath.Join(dir, stressLongName), lines.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeRepeated(t stressT, path string, unit []byte, size int, tail string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	repeats := size / len(unit)
	var wg sync.WaitGroup
	errs := make([]error, stressWriteWorkers)
	for w := range stressWriteWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for r := w; r < repeats; r += stressWriteWorkers {
				if _, err := f.WriteAt(unit, int64(r*len(unit))); err != nil {
					errs[w] = err
					return
				}
			}
		}()
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.WriteAt([]byte(tail), int64(repeats*len(unit))); err != nil {
		t.Fatal(err)
	}
}
