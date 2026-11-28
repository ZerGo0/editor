//go:build e2e

package e2e

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	stressFixtureInsights  = "insights"
	stressFixtureBranches  = "branches"
	stressFixtureWrap      = "wrap"
	stressWideLines        = 100000
	stressWideLineChars    = 160
	stressWideName         = "wide-lines.txt"
	stressTodoFiles        = 2000
	stressTodoPerFile      = 10
	stressTodoFilesPerDir  = 100
	stressTodoCount        = stressTodoFiles * stressTodoPerFile
	stressProblemFiles     = 200
	stressProblemsPerFile  = 50
	stressProblemLines     = 60
	stressProblemCount     = stressProblemFiles * stressProblemsPerFile
	stressBranchCommits    = 50000
	stressBranchFiles      = 10000
	stressBranchFileDirs   = 100
	stressBranchWide       = "wide"
	stressBranchMain       = "main"
	stressBranchHistoryDir = "history"
)

func stressTodoPath(i int) string {
	return fmt.Sprintf("todo/dir%02d/file%04d.go", i/stressTodoFilesPerDir, i)
}

func stressTodoContent(i int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "package todo%d\n\n", i)
	for n := range stressTodoPerFile {
		fmt.Fprintf(&b, "// TODO: item %d of file %d needs work\nfunc f%d() int { return %d }\n\n", n, i, n, n)
	}
	return b.String()
}

func stressProblemPath(i int) string { return fmt.Sprintf("problems/file%03d.go", i) }

func stressProblemContent(i int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "package problem%d\n\n", i)
	for n := range stressProblemLines {
		fmt.Fprintf(&b, "var v%d = (\n", n)
	}
	return b.String()
}

func buildInsightsFixture(t stressT, dir string) {
	writeFilesParallel(t, dir, stressTodoFiles, stressTodoPath, stressTodoContent)
	writeFilesParallel(t, dir, stressProblemFiles, stressProblemPath, stressProblemContent)
}

func stressBranchPath(i int) string {
	return fmt.Sprintf("src/d%02d/c%05d.go", i%stressBranchFileDirs, i)
}

func buildBranchesFixture(t stressT, dir string) {
	stressInitRepo(t, dir)
	stressFastImport(t, dir, func(w io.Writer) {
		fastImportCommit(w, 0, "10k files")
		for i := range stressBranchFiles {
			fastImportFile(w, stressBranchPath(i), stressChangedContent(i))
		}
		for n := 1; n < stressBranchCommits; n++ {
			fastImportCommit(w, n, fmt.Sprintf("commit %d: update history", n))
			fastImportFile(w, fmt.Sprintf("%s/h%02d.txt", stressBranchHistoryDir, n%stressHistoryFiles), fmt.Sprintf("revision %d\n", n))
		}
		message := "rewrite every file"
		fmt.Fprintf(w, "commit refs/heads/%s\ncommitter Ada Lovelace <ada@example.invalid> %d +0000\ndata %d\n%s\nfrom refs/heads/%s\n", stressBranchWide, stressGitDate+stressBranchCommits*stressCommitStepSeconds, len(message), message, stressBranchMain)
		for i := range stressBranchFiles {
			fastImportFile(w, stressBranchPath(i), stressChangedEdit(i))
		}
	})
	stressCheckout(t, dir)
}

func buildWrapFixture(t stressT, dir string) {
	var lines bytes.Buffer
	for i := range stressWideLines {
		line := fmt.Sprintf("line %06d: %s", i, strings.Repeat("the quick brown fox jumps over the lazy dog ", stressWideLineChars/44+1))
		lines.WriteString(line[:stressWideLineChars])
		lines.WriteByte('\n')
	}
	if err := os.WriteFile(filepath.Join(dir, stressWideName), lines.Bytes(), shotFilePerm); err != nil {
		t.Fatal(err)
	}
}
