//go:build e2e

package e2e

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
)

const (
	demoRemoteName     = "origin"
	demoRemoteDir      = "remote.git"
	demoRepoDir        = "demo-project"
	demoIncoming       = 3
	demoOutgoing       = 2
	demoGitDate        = "2026-01-15T12:00:00+0000"
	demoLongOutgoing   = "feat(local): unpushed change with long title"
	demoStashMessage   = "WIP: experimental refactor of the renderer with a message long enough to overflow the row"
	demoStashUntracked = "WIP: scratch notes kept as untracked files"
	demoUntrackedPer   = 160
	demoAddedStaged    = 30
	demoTabCount       = 30
	demoOnlyRemote     = "only-on-the-remote-with-a-long-branch-name-for-remote-tracking-rows"
	demoLongTag        = "very-long-tag-name-for-the-enterprise-edition-stable-release-candidate-number-three"
	demoCacheDirPrefix = "editor-e2e-demo-"
)

var demoTags = []string{"v0.1.0", "v1.0.0", "v1.1.0-beta.2", "release/2026.11.0-rc.1+build.5678", demoLongTag}

type DemoProject struct {
	Root           string
	Remote         string
	Commits        int
	Branches       []string
	Tags           []string
	RemoteBranches []string
	Changed        int
	Conflicted     []string
	TabFiles       []string
	BigFile        string
	ChangelogDir   string
	Ahead          int
	Behind         int
	Stashes        int
}

func (d DemoProject) Path(rel string) string { return filepath.Join(d.Root, filepath.FromSlash(rel)) }

type demoGit struct {
	t   *testing.T
	dir string
	env []string
}

func newDemoGit(t *testing.T, dir string) *demoGit {
	t.Helper()
	env := append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0", "LC_ALL=C",
		"GIT_AUTHOR_NAME=Ada Lovelace", "GIT_AUTHOR_EMAIL=ada@example.invalid", "GIT_AUTHOR_DATE="+demoGitDate,
		"GIT_COMMITTER_NAME=Ada Lovelace", "GIT_COMMITTER_EMAIL=ada@example.invalid", "GIT_COMMITTER_DATE="+demoGitDate,
	)
	return &demoGit{t: t, dir: dir, env: env}
}

func (g *demoGit) exec(stdin []byte, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir, cmd.Env = g.dir, g.env
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func (g *demoGit) run(args ...string) string {
	g.t.Helper()
	out, err := g.exec(nil, args...)
	if err != nil {
		g.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return out
}

func (g *demoGit) stdin(input string, args ...string) {
	g.t.Helper()
	if out, err := g.exec([]byte(input), args...); err != nil {
		g.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func (g *demoGit) commitFile(path, text, message string) {
	g.t.Helper()
	demoWrite(g.t, filepath.Join(g.dir, path), text)
	g.run("add", "--", path)
	g.run("commit", "-q", "-m", message)
}

func demoWrite(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

var demoCache struct {
	once sync.Once
	dir  string
	demo DemoProject
	ok   bool
}

func removeDemoCache() {
	if demoCache.dir != "" {
		_ = os.RemoveAll(demoCache.dir)
	}
}

func NewDemoProject(t *testing.T) DemoProject {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	demoCache.once.Do(func() {
		dir, err := os.MkdirTemp("", demoCacheDirPrefix)
		if err != nil {
			t.Error(err)
			return
		}
		demoCache.dir = dir
		parent, err := filepath.EvalSymlinks(dir)
		if err != nil {
			t.Error(err)
			return
		}
		demoCache.demo = buildDemoProject(t, parent)
		demoCache.ok = true
	})
	if !demoCache.ok {
		t.Fatal("the demo project fixture failed to build")
	}
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return copyDemoProject(t, demoCache.demo, parent)
}

func copyDemoProject(t *testing.T, from DemoProject, parent string) DemoProject {
	t.Helper()
	fromParent := filepath.Dir(from.Root)
	if out, err := exec.Command("cp", "-Rp", filepath.Join(fromParent, demoRepoDir), filepath.Join(fromParent, demoRemoteDir), parent).CombinedOutput(); err != nil {
		t.Fatalf("copy the demo project: %v\n%s", err, out)
	}
	rebase := func(path string) string { return filepath.Join(parent, strings.TrimPrefix(path, fromParent)) }
	d := from
	d.Root, d.Remote, d.BigFile = rebase(from.Root), rebase(from.Remote), rebase(from.BigFile)
	d.TabFiles = make([]string, len(from.TabFiles))
	for i, p := range from.TabFiles {
		d.TabFiles[i] = rebase(p)
	}
	newDemoGit(t, d.Root).run("remote", "set-url", demoRemoteName, d.Remote)
	return d
}

func buildDemoProject(t *testing.T, parent string) DemoProject {
	t.Helper()
	d := DemoProject{Root: filepath.Join(parent, demoRepoDir), Remote: filepath.Join(parent, demoRemoteDir), Commits: demoCommitCount, ChangelogDir: demoChangelogDir}
	if err := os.MkdirAll(d.Root, 0o755); err != nil {
		t.Fatal(err)
	}
	g := newDemoGit(t, d.Root)
	g.run("init", "-q", "-b", "main")
	g.run("config", "user.name", "Ada Lovelace")
	g.run("config", "user.email", "ada@example.invalid")
	g.run("config", "commit.gpgsign", "false")
	g.run("config", "tag.gpgsign", "false")
	g.stdin(string(demoStream()), "fast-import", "--quiet")
	refs := g.run("for-each-ref", "--format=delete %(refname)", "refs/tmp")
	if refs != "" {
		g.stdin(refs+"\n", "update-ref", "--stdin")
	}
	g.run("reset", "-q", "--hard", "main")
	d.tags(g)
	d.remote(g)
	d.stashes(g)
	d.conflict(g)
	d.changes(g)
	d.collect(g)
	return d
}

func (d *DemoProject) tags(g *demoGit) {
	for i, tag := range demoTags {
		target := fmt.Sprintf("main~%d", (len(demoTags)-i)*300)
		if i%2 == 0 {
			g.run("tag", "-a", tag, "-m", "Release "+tag, target)
		} else {
			g.run("tag", tag, target)
		}
	}
}

func (d *DemoProject) remote(g *demoGit) {
	bare := newDemoGit(g.t, filepath.Dir(d.Remote))
	bare.run("init", "-q", "--bare", "-b", "main", d.Remote)
	g.run("remote", "add", demoRemoteName, d.Remote)
	g.run("push", "-q", "--tags", demoRemoteName, "main", demoFeatureBranch, demoBugfixBranch, demoReleaseBranch)
	g.run("branch", "-q", "--set-upstream-to="+demoRemoteName+"/main", "main")
	g.run("branch", "-q", "--set-upstream-to="+demoRemoteName+"/"+demoFeatureBranch, demoFeatureBranch)
	g.run("push", "-q", demoRemoteName, demoDevelopBranch+":refs/heads/"+demoOnlyRemote)
	g.run("checkout", "-q", "-b", "incoming", "main")
	for i := range demoIncoming {
		g.commitFile(fmt.Sprintf("incoming/remote-change-%d.txt", i), fmt.Sprintf("change %d from a teammate\n", i), fmt.Sprintf("feat(remote): teammate change %d", i))
	}
	g.run("push", "-q", demoRemoteName, "incoming:main")
	g.run("checkout", "-q", "main")
	g.run("branch", "-q", "-D", "incoming")
	g.run("fetch", "-q", demoRemoteName)
	for i := range demoOutgoing {
		subject := fmt.Sprintf("feat(local): unpushed change %d", i)
		if i == demoOutgoing-1 {
			subject = demoLongOutgoing
		}
		g.commitFile(fmt.Sprintf("outgoing/local-change-%d.txt", i), fmt.Sprintf("local change %d\n", i), subject)
	}
}

func (d *DemoProject) stashes(g *demoGit) {
	demoWrite(g.t, d.Path("README.md"), "# Demo project\n\nStashed experiment.\n")
	g.run("stash", "push", "-q", "-m", demoStashMessage)
	demoWrite(g.t, d.Path("scratch-notes/stashed.md"), "untracked notes\n")
	demoWrite(g.t, d.Path(demoVersionFile), demoVersionText(9999))
	g.run("stash", "push", "-q", "-u", "-m", demoStashUntracked)
}

func (d *DemoProject) conflict(g *demoGit) {
	if out, err := g.exec(nil, "merge", "--no-commit", "--no-ff", demoConflictBranch); err == nil {
		g.t.Fatalf("merge produced no conflict:\n%s", out)
	}
}

func (d *DemoProject) changes(g *demoGit) {
	var modStaged, modBoth, delStaged, renamed, addedStaged []string
	for k, f := range demoModuleCatalog() {
		path := d.Path(f.Path)
		switch k % 10 {
		case 0, 1, 2:
			demoWrite(g.t, path, f.Text+"// edited, not staged\n")
		case 3, 4:
			demoWrite(g.t, path, f.Text+"// edited and staged\n")
			modStaged = append(modStaged, f.Path)
		case 5:
			demoWrite(g.t, path, f.Text+"// staged part\n")
			modBoth = append(modBoth, f.Path)
		case 6:
			if k%20 == 6 {
				if err := os.Remove(path); err != nil {
					g.t.Fatal(err)
				}
			} else {
				delStaged = append(delStaged, f.Path)
			}
		case 7:
			target := fmt.Sprintf("src/modules/renamed-by-refactor/mod%02d-file%02d.go", k/(len(demoModuleSubs)*demoModuleFiles), k%demoModuleFiles)
			if err := os.MkdirAll(filepath.Dir(d.Path(target)), 0o755); err != nil {
				g.t.Fatal(err)
			}
			if err := os.Rename(path, d.Path(target)); err != nil {
				g.t.Fatal(err)
			}
			renamed = append(renamed, f.Path, target)
		}
	}
	for i := range demoAddedStaged {
		path := fmt.Sprintf("src/new/feature-%02d.go", i)
		demoWrite(g.t, d.Path(path), fmt.Sprintf("package new\n\n// TODO: finish feature %d\n", i))
		addedStaged = append(addedStaged, path)
	}
	g.stdin(strings.Join(slices.Concat(modStaged, modBoth, addedStaged), "\n")+"\n", "add", "--pathspec-from-file=-")
	g.stdin(strings.Join(delStaged, "\n")+"\n", "rm", "-q", "--pathspec-from-file=-")
	g.stdin(strings.Join(renamed, "\n")+"\n", "add", "-A", "--pathspec-from-file=-")
	for k, f := range demoModuleCatalog() {
		if k%10 == 5 {
			demoWrite(g.t, d.Path(f.Path), f.Text+"// staged part\n// unstaged part\n")
		}
	}
	for i := range demoUntrackedPer {
		demoWrite(g.t, d.Path(fmt.Sprintf("scratch/experiment-%02d/notes-%03d.md", i%16, i)), fmt.Sprintf("# Scratch %d\n\nTODO: clean up\n", i))
	}
	demoWrite(g.t, d.Path("scratch/a-really-long-untracked-file-name-that-does-not-fit-in-the-sidebar-at-all-ever.txt"), "x\n")
	demoWrite(g.t, d.Path("build/output.bin"), "ignored\n")
	demoWrite(g.t, d.Path("node_modules/pkg/index.js"), "ignored\n")
	demoWrite(g.t, d.Path("debug.log"), "ignored\n")
}

func (d *DemoProject) collect(g *demoGit) {
	d.Branches = lines(g.run("for-each-ref", "--format=%(refname:short)", "refs/heads"))
	d.Tags = lines(g.run("tag", "--list"))
	d.RemoteBranches = lines(g.run("for-each-ref", "--format=%(refname:short)", "refs/remotes"))
	d.Changed = len(lines(g.run("status", "--porcelain=v1", "--untracked-files=all")))
	d.Conflicted = lines(g.run("diff", "--name-only", "--diff-filter=U"))
	d.BigFile = d.Path(demoBigFile)
	d.Stashes = len(lines(g.run("stash", "list")))
	var ahead, behind int
	if _, err := fmt.Sscanf(g.run("rev-list", "--left-right", "--count", "main...origin/main"), "%d %d", &ahead, &behind); err != nil {
		g.t.Fatal(err)
	}
	d.Ahead, d.Behind = ahead, behind
	var tabs []string
	for _, rel := range []string{"web/app/" + demoDuplicateName, "web/components/button/" + demoDuplicateName, "web/lib/" + demoDuplicateName, "internal/auth/utils.go", "internal/billing/utils.go", "internal/render/utils.go", "config.json", "docs/config.json", "web/config.json", "README.md", "docs/README.md", "web/README.md"} {
		tabs = append(tabs, d.Path(rel))
	}
	for _, f := range demoProblemFiles() {
		tabs = append(tabs, d.Path(f.Path))
	}
	for _, f := range demoLanguageSamples() {
		if len(tabs) == demoTabCount {
			break
		}
		tabs = append(tabs, d.Path(f.Path))
	}
	d.TabFiles = tabs
}

func lines(out string) []string {
	if out == "" {
		return nil
	}
	return strings.Split(out, "\n")
}

func TestDemoProjectFixture(t *testing.T) {
	t.Parallel()
	d := NewDemoProject(t)
	g := newDemoGit(t, d.Root)
	var commits int
	if _, err := fmt.Sscanf(g.run("rev-list", "--count", "main"), "%d", &commits); err != nil || commits < 2000 {
		t.Fatalf("history has %d commits (%v), want at least 2000", commits, err)
	}
	if d.Changed < 500 {
		t.Fatalf("%d changed files, want at least 500", d.Changed)
	}
	status := g.run("status", "--porcelain=v1", "--untracked-files=all")
	for _, kind := range []struct{ name, code string }{
		{"staged modification", "M  "}, {"unstaged modification", " M "}, {"staged and unstaged", "MM "}, {"unstaged deletion", " D "},
		{"staged deletion", "D  "}, {"staged rename", "R  "}, {"staged addition", "A  "}, {"untracked", "?? "}, {"conflict", "UU "},
	} {
		if !hasStatus(status, kind.code) {
			t.Errorf("no %s entry (%q) in status", kind.name, kind.code)
		}
	}
	if len(d.Conflicted) < 2 {
		t.Errorf("conflicted files = %v", d.Conflicted)
	}
	if len(d.Branches) < 6 || len(d.Tags) < 5 || len(d.RemoteBranches) < 4 {
		t.Errorf("branches %d, tags %d, remote branches %d", len(d.Branches), len(d.Tags), len(d.RemoteBranches))
	}
	if !slices.ContainsFunc(d.Branches, func(b string) bool { return len(b) > 60 }) || !slices.ContainsFunc(d.Tags, func(b string) bool { return len(b) > 60 }) {
		t.Errorf("no long branch or tag name: %v %v", d.Branches, d.Tags)
	}
	if d.Ahead != demoOutgoing || d.Behind != demoIncoming {
		t.Errorf("main is ahead %d behind %d, want %d and %d", d.Ahead, d.Behind, demoOutgoing, demoIncoming)
	}
	if d.Stashes != 2 {
		t.Errorf("stashes = %d, want 2", d.Stashes)
	}
	if out := g.run("grep", "-c", "-E", "TODO|FIXME|HACK", "--", demoBigFile); out == "" {
		t.Errorf("no TODO comments in %s", demoBigFile)
	}
	if info, err := os.Stat(d.BigFile); err != nil || info.Size() == 0 {
		t.Errorf("big file: %v", err)
	}
	if len(d.TabFiles) != demoTabCount {
		t.Errorf("tab files = %d, want %d", len(d.TabFiles), demoTabCount)
	}
	for _, path := range d.TabFiles {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("tab file: %v", err)
		}
	}
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	again := buildDemoProject(t, parent)
	if a, b := g.run("rev-parse", "main", "origin/main", "stash@{0}^{tree}"), newDemoGit(t, again.Root).run("rev-parse", "main", "origin/main", "stash@{0}^{tree}"); a != b {
		t.Errorf("fixture is not deterministic:\n%s\n%s", a, b)
	}
	if again.Changed != d.Changed {
		t.Errorf("changed files differ between runs: %d and %d", again.Changed, d.Changed)
	}
}

func hasStatus(status, code string) bool {
	for _, line := range lines(status) {
		if strings.HasPrefix(line, code) {
			return true
		}
	}
	return false
}
