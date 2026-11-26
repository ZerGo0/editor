package git

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestParseRemotes(t *testing.T) {
	text := "origin\thttps://example.com/a.git (fetch)\norigin\thttps://example.com/a.git (push)\n" +
		"fork\tgit@example.com:me/a.git (fetch)\nfork\tgit@example.com:me/push.git (push)"
	got, err := parseRemotes(text)
	if err != nil {
		t.Fatal(err)
	}
	want := []Remote{
		{Name: "origin", FetchURL: "https://example.com/a.git", PushURL: "https://example.com/a.git"},
		{Name: "fork", FetchURL: "git@example.com:me/a.git", PushURL: "git@example.com:me/push.git"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
	for _, bad := range []string{"origin url (fetch)", "origin\turl (sideways)"} {
		if _, err := parseRemotes(bad); err == nil {
			t.Errorf("expected error for %q", bad)
		}
	}
}

func TestLineSplitter(t *testing.T) {
	var lines []string
	s := &lineSplitter{emit: func(l string) { lines = append(lines, l) }}
	for _, chunk := range []string{"Counting 10%\rCounting 50%\rCount", "ing 100%\nResolving\r\n", "tail"} {
		if _, err := s.Write([]byte(chunk)); err != nil {
			t.Fatal(err)
		}
	}
	s.flush()
	want := []string{"Counting 10%", "Counting 50%", "Counting 100%", "Resolving", "tail"}
	if !reflect.DeepEqual(lines, want) {
		t.Fatalf("got %q", lines)
	}
}

type remoteSetup struct {
	repo   *Repo
	remote string
}

func setupRemote(t *testing.T) remoteSetup {
	t.Helper()
	remote := newBareRemote(t)
	repo := newRepo(t)
	commitFile(t, repo, "a.txt", "a\n", "init")
	g(t, repo, "remote", "add", "origin", remote)
	if err := repo.Push(context.Background(), PushOptions{Remote: "origin", SetUpstream: true}); err != nil {
		t.Fatalf("initial push: %v", err)
	}
	return remoteSetup{repo: repo, remote: remote}
}

func cloneOf(t *testing.T, remote string) *Repo {
	t.Helper()
	dir := t.TempDir()
	rawGit(t, dir, "", "clone", "--quiet", remote, "clone")
	repo, err := Open(context.Background(), dir+"/clone")
	if err != nil {
		t.Fatal(err)
	}
	return repo
}

func TestRemotesListing(t *testing.T) {
	s := setupRemote(t)
	remotes, err := s.repo.Remotes(context.Background())
	if err != nil || len(remotes) != 1 || remotes[0].Name != "origin" || remotes[0].FetchURL != s.remote || remotes[0].PushURL != s.remote {
		t.Fatalf("remotes: %+v %v", remotes, err)
	}
}

func TestPushFetchPull(t *testing.T) {
	ctx := context.Background()
	s := setupRemote(t)
	repo := s.repo
	branches, _ := repo.Branches(ctx)
	if len(branches) != 2 {
		t.Fatalf("expected local and remote branch: %+v", branches)
	}
	for _, b := range branches {
		if !b.Remote && (b.Upstream != "origin/main" || b.Ahead != 0 || b.Behind != 0) {
			t.Fatalf("tracking: %+v", b)
		}
	}
	commitFile(t, repo, "local.txt", "l\n", "local commit")
	out, err := repo.OutgoingCommits(ctx)
	if err != nil || len(out) != 1 || out[0].Subject != "local commit" {
		t.Fatalf("outgoing: %+v %v", out, err)
	}
	st, _ := repo.Status(ctx)
	if st.Ahead != 1 || st.Behind != 0 || st.Upstream != "origin/main" {
		t.Fatalf("status ahead: %+v", st)
	}
	var progress []string
	if err := repo.Push(ctx, PushOptions{Progress: func(l string) { progress = append(progress, l) }}); err != nil {
		t.Fatal(err)
	}
	if out, _ = repo.OutgoingCommits(ctx); len(out) != 0 {
		t.Fatalf("outgoing after push: %+v", out)
	}

	other := cloneOf(t, s.remote)
	commitFile(t, other, "remote.txt", "r\n", "remote commit")
	if err := other.Push(ctx, PushOptions{}); err != nil {
		t.Fatal(err)
	}
	var fetchLines []string
	if err := repo.Fetch(ctx, "origin", true, func(l string) { fetchLines = append(fetchLines, l) }); err != nil {
		t.Fatal(err)
	}
	in, err := repo.IncomingCommits(ctx)
	if err != nil || len(in) != 1 || in[0].Subject != "remote commit" {
		t.Fatalf("incoming: %+v %v", in, err)
	}
	branches, _ = repo.Branches(ctx)
	for _, b := range branches {
		if b.Name == "main" && b.Behind != 1 {
			t.Fatalf("behind: %+v", b)
		}
	}
	res, err := repo.Pull(ctx, PullOptions{FFOnly: true})
	if err != nil || res.Stopped || !exists(repo, "remote.txt") {
		t.Fatalf("ff pull: %+v %v", res, err)
	}

	commitFile(t, repo, "mine.txt", "m\n", "mine")
	commitFile(t, other, "theirs.txt", "t\n", "theirs")
	if err := other.Push(ctx, PushOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := repo.Fetch(ctx, "", false, nil); err != nil {
		t.Fatal(err)
	}
	if err := repo.Push(ctx, PushOptions{}); err == nil {
		t.Fatal("non-fast-forward push must fail")
	} else {
		var ge *Error
		if !errors.As(err, &ge) || ge.ExitCode == 0 {
			t.Fatalf("push error: %v", err)
		}
	}
	if _, err := repo.Pull(ctx, PullOptions{FFOnly: true}); err == nil {
		t.Fatal("ff-only pull of diverged branch must fail")
	}
	if _, err := repo.Pull(ctx, PullOptions{FFOnly: true, Rebase: true}); err == nil {
		t.Fatal("exclusive options must fail")
	}
	res, err = repo.Pull(ctx, PullOptions{Rebase: true})
	if err != nil || res.Stopped {
		t.Fatalf("rebase pull: %+v %v", res, err)
	}
	if got := g(t, repo, "rev-list", "--merges", "--count", "HEAD"); got != "0" {
		t.Fatalf("rebase pull created merges: %s", got)
	}
	if err := repo.Push(ctx, PushOptions{}); err != nil {
		t.Fatal(err)
	}
}

func TestPullMergeAndConflict(t *testing.T) {
	ctx := context.Background()
	s := setupRemote(t)
	repo := s.repo
	other := cloneOf(t, s.remote)
	commitFile(t, other, "a.txt", "theirs\n", "theirs edit")
	commitFile(t, other, "t.txt", "t\n", "theirs file")
	if err := other.Push(ctx, PushOptions{}); err != nil {
		t.Fatal(err)
	}
	commitFile(t, repo, "m.txt", "m\n", "mine")
	res, err := repo.Pull(ctx, PullOptions{})
	if err != nil || res.Stopped {
		t.Fatalf("merge pull: %+v %v", res, err)
	}
	if got := g(t, repo, "rev-list", "--merges", "--count", "HEAD"); got != "1" {
		t.Fatalf("merge pull should create a merge: %s", got)
	}
	if err := repo.Push(ctx, PushOptions{}); err != nil {
		t.Fatal(err)
	}
	g(t, other, "pull", "--quiet", "--no-rebase")
	commitFile(t, other, "a.txt", "theirs 2\n", "theirs again")
	if err := other.Push(ctx, PushOptions{}); err != nil {
		t.Fatal(err)
	}
	commitFile(t, repo, "a.txt", "mine 2\n", "mine again")
	res, err = repo.Pull(ctx, PullOptions{})
	if err != nil || !res.Stopped {
		t.Fatalf("conflicting pull must stop: %+v %v", res, err)
	}
	state, _ := repo.State(ctx)
	if state.Kind != StateMerging {
		t.Fatalf("state: %+v", state)
	}
	if err := repo.Abort(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestPushForceAndTags(t *testing.T) {
	ctx := context.Background()
	s := setupRemote(t)
	repo := s.repo
	g(t, repo, "tag", "v1")
	if err := repo.Push(ctx, PushOptions{Remote: "origin", Branch: "main", Tags: true}); err != nil {
		t.Fatal(err)
	}
	if got := rawGit(t, s.remote, "", "tag", "--list"); got != "v1" {
		t.Fatalf("remote tags: %q", got)
	}
	other := cloneOf(t, s.remote)
	commitFile(t, other, "x.txt", "x\n", "remote only")
	if err := other.Push(ctx, PushOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := repo.Fetch(ctx, "origin", false, nil); err != nil {
		t.Fatal(err)
	}
	g(t, repo, "commit", "--quiet", "--allow-empty", "-m", "diverge")
	if err := repo.Push(ctx, PushOptions{Force: true}); err != nil {
		t.Fatalf("force-with-lease after fetch: %v", err)
	}
	if got := rawGit(t, s.remote, "", "log", "-1", "--format=%s", "main"); got != "diverge" {
		t.Fatalf("remote head: %q", got)
	}
	if err := other.Fetch(ctx, "origin", false, nil); err != nil {
		t.Fatal(err)
	}
	commitFile(t, other, "y.txt", "y\n", "sneaky")
	if err := other.Push(ctx, PushOptions{Force: true}); err != nil {
		t.Fatal(err)
	}
	g(t, repo, "commit", "--quiet", "--allow-empty", "-m", "stale")
	if err := repo.Push(ctx, PushOptions{Force: true}); err == nil {
		t.Fatal("force-with-lease must refuse when the remote moved after the last fetch")
	}
}

func TestOutgoingWithoutUpstream(t *testing.T) {
	ctx := context.Background()
	s := setupRemote(t)
	repo := s.repo
	g(t, repo, "switch", "--quiet", "--create", "feature")
	commitFile(t, repo, "f.txt", "f\n", "feature commit")
	out, err := repo.OutgoingCommits(ctx)
	if err != nil || len(out) != 1 || out[0].Subject != "feature commit" {
		t.Fatalf("outgoing: %+v %v", out, err)
	}
	if _, err := repo.IncomingCommits(ctx); !errors.Is(err, ErrNoUpstream) {
		t.Fatalf("incoming: %v", err)
	}
	if err := repo.Push(ctx, PushOptions{Remote: "origin", SetUpstream: true}); err != nil {
		t.Fatal(err)
	}
	st, _ := repo.Status(ctx)
	if st.Upstream != "origin/feature" {
		t.Fatalf("upstream: %+v", st)
	}
	other := cloneOf(t, s.remote)
	if err := other.CheckoutNewTracking(ctx, "origin/feature"); err != nil {
		t.Fatal(err)
	}
	if got := g(t, other, "rev-parse", "--abbrev-ref", "feature@{upstream}"); got != "origin/feature" {
		t.Fatalf("tracking: %q", got)
	}
	if err := other.CheckoutNewTracking(ctx, "nodash"); err == nil {
		t.Fatal("name without remote must fail")
	}
}
