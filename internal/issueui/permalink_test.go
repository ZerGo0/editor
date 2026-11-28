package issueui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/testutil"
	"github.com/egoist/mygo/ui/editor"
)

func gitTestEnv(t *testing.T) {
	t.Helper()
	if !git.Available() {
		t.Skip("git is not installed")
	}
	home := t.TempDir()
	for k, v := range map[string]string{
		"HOME": home, "XDG_CONFIG_HOME": home, "GIT_CONFIG_GLOBAL": os.DevNull, "GIT_CONFIG_NOSYSTEM": "1",
		"GIT_AUTHOR_NAME": "Ada", "GIT_AUTHOR_EMAIL": "ada@example.com", "GIT_COMMITTER_NAME": "Ada", "GIT_COMMITTER_EMAIL": "ada@example.com",
	} {
		t.Setenv(k, v)
	}
}

func newPushedRepo(t *testing.T) (string, Local, string) {
	t.Helper()
	gitTestEnv(t)
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	testutil.RunGit(t, dir, "init", "--quiet", "-b", "main")
	testutil.WriteFile(t, filepath.Join(dir, "src/a.go"), "one\ntwo\nthree\n")
	testutil.WriteFile(t, filepath.Join(dir, "dir with space/#x.go"), "x\n")
	testutil.RunGit(t, dir, "add", ".")
	testutil.RunGit(t, dir, "commit", "--quiet", "-m", "first")
	testutil.RunGit(t, dir, "remote", "add", "origin", "https://github.com/octo/repo.git")
	testutil.RunGit(t, dir, "update-ref", "refs/remotes/origin/main", "HEAD")
	testutil.RunGit(t, dir, "branch", "--set-upstream-to=origin/main", "main")
	repo, err := git.Open(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	return dir, cliLocal{repo}, testutil.RunGit(t, dir, "rev-parse", "HEAD")
}

func TestPermalinkWithRealGit(t *testing.T) {
	dir, local, sha := newPushedRepo(t)
	ctx := context.Background()
	base := "https://github.com/octo/repo/blob/" + sha + "/"
	tests := []struct {
		name       string
		path       string
		start, end int
		want       string
	}{
		{"single line", filepath.Join(dir, "src/a.go"), 1, 1, base + "src/a.go#L2"},
		{"range", filepath.Join(dir, "src/a.go"), 1, 2, base + "src/a.go#L2-L3"},
		{"reversed range", filepath.Join(dir, "src/a.go"), 2, 0, base + "src/a.go#L1-L3"},
		{"whole file", filepath.Join(dir, "src/a.go"), -1, -1, base + "src/a.go"},
		{"relative path", "src/a.go", 0, 0, base + "src/a.go#L1"},
		{"escaped path", filepath.Join(dir, "dir with space/#x.go"), 0, 0, base + "dir%20with%20space/%23x.go#L1"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := buildPermalink(ctx, local, tc.path, tc.start, tc.end)
			if err != nil || got != tc.want {
				t.Errorf("got %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}

func TestPermalinkRefusesUnsafeStates(t *testing.T) {
	dir, local, _ := newPushedRepo(t)
	ctx := context.Background()
	testutil.WriteFile(t, filepath.Join(dir, "src/a.go"), "changed\n")
	testutil.WriteFile(t, filepath.Join(dir, "new.go"), "new\n")
	if _, err := buildPermalink(ctx, local, filepath.Join(dir, "src/a.go"), 0, 0); err != errPermalinkModified {
		t.Errorf("modified file: %v", err)
	}
	if _, err := buildPermalink(ctx, local, filepath.Join(dir, "new.go"), 0, 0); err != errPermalinkUntracked {
		t.Errorf("untracked file: %v", err)
	}
	if got, err := buildPermalink(ctx, local, filepath.Join(dir, "dir with space/#x.go"), 0, 0); err != nil || got == "" {
		t.Errorf("an unmodified file must still work: %q %v", got, err)
	}
	if _, err := buildPermalink(ctx, local, filepath.Join(filepath.Dir(dir), "elsewhere.go"), 0, 0); err != errPermalinkOutside {
		t.Errorf("outside file: %v", err)
	}
	testutil.RunGit(t, dir, "checkout", "--quiet", "--", "src/a.go")
	testutil.RunGit(t, dir, "commit", "--quiet", "--allow-empty", "-m", "local only")
	if _, err := buildPermalink(ctx, local, filepath.Join(dir, "src/a.go"), 0, 0); err != errPermalinkUnpushed {
		t.Errorf("unpushed commit: %v", err)
	}
}

func TestPermalinkFakeFailures(t *testing.T) {
	ctx := context.Background()
	local := newFakeLocal()
	path := testRoot + "/src/a.go"
	local.status.OID = ""
	if _, err := buildPermalink(ctx, local, path, 0, 0); err != errPermalinkNoCommit {
		t.Errorf("no commit: %v", err)
	}
	local = newFakeLocal()
	local.remotes = []git.Remote{{Name: "origin", FetchURL: "/srv/git/repo.git"}}
	if _, err := buildPermalink(ctx, local, path, 0, 0); err != errPermalinkNoRemote {
		t.Errorf("no github remote: %v", err)
	}
	local = newFakeLocal()
	local.errs["Status"] = errBoom
	if _, err := buildPermalink(ctx, local, path, 0, 0); !errors.Is(err, errBoom) {
		t.Errorf("status failure: %v", err)
	}
	local = newFakeLocal()
	local.errs["OutgoingCommits"] = errBoom
	if _, err := buildPermalink(ctx, local, path, 0, 0); !errors.Is(err, errBoom) {
		t.Errorf("outgoing failure: %v", err)
	}
	local = newFakeLocal()
	local.status.Files = []git.FileStatus{{Path: "old.go", OrigPath: "src/a.go", Index: 'R', WorkTree: ' '}}
	if _, err := buildPermalink(ctx, local, path, 0, 0); err != errPermalinkModified {
		t.Errorf("renamed file: %v", err)
	}
}

func TestPermalinkRemoteChoice(t *testing.T) {
	remotes := []git.Remote{
		{Name: "fork", FetchURL: "git@github.com:me/widgets.git"},
		{Name: "upstream", FetchURL: "git@github.com:acme/widgets.git"},
		{Name: "origin", FetchURL: "git@github.com:me/widgets-origin.git"},
	}
	tests := []struct {
		upstream, want string
	}{
		{"fork/topic", "me/widgets"},
		{"", "me/widgets-origin"},
		{"gone/topic", "me/widgets-origin"},
	}
	for _, tc := range tests {
		got, ok := permalinkRemote(remotes, tc.upstream)
		if !ok || got.Owner+"/"+got.Name != tc.want {
			t.Errorf("upstream %q: %+v %v", tc.upstream, got, ok)
		}
	}
	got, ok := permalinkRemote(remotes[:1], "")
	if !ok || got.Name != "widgets" {
		t.Errorf("single remote: %+v %v", got, ok)
	}
	if _, ok := permalinkRemote(nil, ""); ok {
		t.Error("no remotes must fail")
	}
}

func TestPermalinkActions(t *testing.T) {
	hs := newHarness(t)
	hs.ready(t)
	hs.i.Run(ActionCopyPermalink)
	if hs.h.LastNotice() != "Open a file in the editor first." {
		t.Errorf("notice %q", hs.h.LastNotice())
	}
	hs.h.Path = testRoot + "/src/a.go"
	hs.h.State = &editor.State{Buffer: editor.NewBuffer("one\ntwo\nthree\n")}
	hs.h.State.SetSelections(editor.Selection{Anchor: 4, Head: 7})
	hs.i.Run(ActionCopyPermalink)
	hs.h.Pump(t, func() bool { return len(hs.copied) == 1 })
	want := "https://github.com/acme/widgets/blob/abc123/src/a.go#L2"
	if hs.copied[0] != want || hs.h.LastNotice() != "Copied the GitHub permalink" {
		t.Errorf("copied %v notice %q", hs.copied, hs.h.LastNotice())
	}
	hs.i.Run(ActionOpenPermalink)
	hs.h.Pump(t, func() bool { return len(hs.urls) == 1 })
	if hs.urls[0] != want {
		t.Errorf("opened %v", hs.urls)
	}
	hs.local.mu.Lock()
	hs.local.outgoing = nil
	hs.local.status.Files = []git.FileStatus{{Path: "src/a.go", Index: ' ', WorkTree: 'M'}}
	hs.local.mu.Unlock()
	hs.i.CopyPermalink(hs.h.Path, 0, 0)
	hs.h.Pump(t, func() bool {
		return len(hs.h.Errs) == 1 && strings.HasSuffix(hs.h.Errs[0], errPermalinkModified.Error())
	})
	if len(hs.copied) != 1 {
		t.Error("a failed permalink was copied")
	}
}

func TestPermalinkWorksWithoutConnection(t *testing.T) {
	hs := newHarness(t)
	hs.token = errBoom
	hs.start(t)
	link, err := hs.i.Permalink(testRoot+"/src/a.go", 0, 0)
	if err != nil || link != "https://github.com/acme/widgets/blob/abc123/src/a.go#L1" {
		t.Errorf("%q %v", link, err)
	}
	hs.open = git.ErrNotRepo
	if _, err := hs.i.Permalink(testRoot+"/src/a.go", 0, 0); !errors.Is(err, git.ErrNotRepo) {
		t.Errorf("open failure: %v", err)
	}
}

func TestSelectionLines(t *testing.T) {
	buf := editor.NewBuffer("one\ntwo\nthree\nfour\n")
	tests := []struct {
		sel        editor.Selection
		start, end int
	}{
		{editor.Selection{Anchor: 5, Head: 5}, 1, 1},
		{editor.Selection{Anchor: 4, Head: 11}, 1, 2},
		{editor.Selection{Anchor: 4, Head: 8}, 1, 1},
		{editor.Selection{Anchor: 11, Head: 4}, 1, 2},
		{editor.Selection{Anchor: 0, Head: 19}, 0, 3},
	}
	for _, tc := range tests {
		start, end := selectionLines(buf, tc.sel)
		if start != tc.start || end != tc.end {
			t.Errorf("selection %+v: %d-%d, want %d-%d", tc.sel, start, end, tc.start, tc.end)
		}
	}
}
