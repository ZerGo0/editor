package gitui

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/ZerGo0/editor/internal/git"
	"github.com/egoist/mygo"
)

const (
	shelfHashLength = 16
	shelfPatchName  = "patch.diff"
)

var shelfDataDirs = []string{"data", "shelf"}

type repo interface {
	WorkDir() string
	MetaDir() string
	Status(ctx context.Context) (*git.Status, error)
	State(ctx context.Context) (git.RepoState, error)
	Branches(ctx context.Context) ([]git.Branch, error)
	Remotes(ctx context.Context) ([]git.Remote, error)
	Log(ctx context.Context, o git.LogOptions) ([]git.Commit, error)
	CommitFiles(ctx context.Context, oid string) ([]git.ChangedFile, error)
	Show(ctx context.Context, rev, path string) ([]byte, error)
	Commit(ctx context.Context, o git.CommitOptions) (string, error)
	LastCommitMessage(ctx context.Context) (string, error)
	StageFiles(ctx context.Context, paths ...string) error
	UnstageFiles(ctx context.Context, paths ...string) error
	Discard(ctx context.Context, paths ...string) error
	DiscardUntracked(ctx context.Context, paths ...string) error
	Apply(ctx context.Context, patch []byte, cached, reverse bool) error
	Fetch(ctx context.Context, remote string, prune bool, progress func(line string)) error
	Pull(ctx context.Context, o git.PullOptions) (git.OpResult, error)
	Push(ctx context.Context, o git.PushOptions) error
	OutgoingCommits(ctx context.Context) ([]git.Commit, error)
	Checkout(ctx context.Context, ref string, force bool) error
	CheckoutNewTracking(ctx context.Context, remoteBranch string) error
	CreateBranch(ctx context.Context, name, start string, checkout bool) error
	RenameBranch(ctx context.Context, oldName, newName string) error
	DeleteBranch(ctx context.Context, name string, force bool) error
	Merge(ctx context.Context, ref string, o git.MergeOptions) (git.OpResult, error)
	Rebase(ctx context.Context, onto string) (git.OpResult, error)
	CompareBranches(ctx context.Context, a, b string) (git.BranchComparison, error)
	CherryPick(ctx context.Context, oids ...string) (git.OpResult, error)
	Revert(ctx context.Context, oids ...string) (git.OpResult, error)
	Reset(ctx context.Context, rev string, mode git.ResetMode) error
	RebaseTodo(ctx context.Context, onto string) ([]git.TodoItem, error)
	RebaseInteractive(ctx context.Context, onto string, items []git.TodoItem, messages map[string]string) (git.OpResult, error)
	Continue(ctx context.Context) (git.OpResult, error)
	Skip(ctx context.Context) (git.OpResult, error)
	Abort(ctx context.Context) error
	Conflicts(ctx context.Context) ([]git.ConflictFile, error)
	AcceptSide(ctx context.Context, path string, ours bool) error
	MergeVersions(ctx context.Context, path string) (base, ours, theirs []byte, err error)
	MarkResolved(ctx context.Context, paths ...string) error
	Blame(ctx context.Context, path, rev string, contents []byte) ([]git.BlameLine, error)
	LineLog(ctx context.Context, path string, start, end int) ([]git.LineLogEntry, error)
	DiffCommits(ctx context.Context, a, b string, paths ...string) ([]git.FileDiff, error)
	DiffWorkTree(ctx context.Context, paths ...string) ([]git.FileDiff, error)
	DiffStaged(ctx context.Context, paths ...string) ([]git.FileDiff, error)
	Stashes(ctx context.Context) ([]git.Stash, error)
	StashPush(ctx context.Context, message string, includeUntracked, keepIndex bool, paths ...string) (bool, error)
	StashApply(ctx context.Context, index int, pop bool) (git.OpResult, error)
	StashDrop(ctx context.Context, index int) error
	StashShow(ctx context.Context, index int) ([]git.FileDiff, error)
	Worktrees(ctx context.Context) ([]git.Worktree, error)
	AddWorktree(ctx context.Context, path, branch string, newBranch bool) error
	RemoveWorktree(ctx context.Context, path string, force bool) error
	Submodules(ctx context.Context) ([]git.Submodule, error)
	UpdateSubmodules(ctx context.Context, initialize, recursive bool) error
	Tags(ctx context.Context) ([]git.Tag, error)
	CreateTag(ctx context.Context, name, rev, message string) error
	DeleteTag(ctx context.Context, name string) error
	PushRef(ctx context.Context, remote, refspec string) error
	AddRemote(ctx context.Context, name, url string) error
	SetRemoteURL(ctx context.Context, name, url string) error
	RenameRemote(ctx context.Context, oldName, newName string) error
	RemoveRemote(ctx context.Context, name string) error
	BranchesContaining(ctx context.Context, oid string) ([]string, error)
	ChangedSince(ctx context.Context, rev string) ([]git.ChangedFile, error)
	Shelve(ctx context.Context, name string, paths []string) (git.ShelfEntry, error)
	Unshelve(ctx context.Context, id string, remove bool) (git.OpResult, error)
	ShelfList() ([]git.ShelfEntry, error)
	ShelfDelete(id string) error
	ShelfPatch(id string) ([]byte, error)
	ShelfUsage() (int64, error)
}

type cliRepo struct {
	*git.Repo
	shelf git.Shelf
}

func (r *cliRepo) WorkDir() string { return r.Root }

func (r *cliRepo) MetaDir() string { return r.GitDir }

func (r *cliRepo) Shelve(ctx context.Context, name string, paths []string) (git.ShelfEntry, error) {
	return r.shelf.Shelve(ctx, r.Repo, name, paths, false)
}

func (r *cliRepo) Unshelve(ctx context.Context, id string, remove bool) (git.OpResult, error) {
	return r.shelf.Unshelve(ctx, r.Repo, id, remove)
}

func (r *cliRepo) ShelfList() ([]git.ShelfEntry, error) { return r.shelf.List() }

func (r *cliRepo) ShelfDelete(id string) error { return r.shelf.Delete(id) }

func (r *cliRepo) ShelfPatch(id string) ([]byte, error) {
	if !filepath.IsLocal(id) || strings.ContainsAny(id, "/\\") {
		return nil, fmt.Errorf("shelf: invalid id %q", id)
	}
	data, err := os.ReadFile(filepath.Join(r.shelf.Dir, id, shelfPatchName))
	if err != nil {
		return nil, fmt.Errorf("shelf: read %s: %w", id, err)
	}
	return data, nil
}

func (r *cliRepo) ShelfUsage() (int64, error) {
	return dirSize(r.shelf.Dir)
}

func dirSize(dir string) (int64, error) {
	var total int64
	err := filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if d.Type().IsRegular() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			total += info.Size()
		}
		return nil
	})
	return total, err
}

type deps struct {
	available func() bool
	open      func(ctx context.Context, dir, dataDir string) (repo, error)
	initRepo  func(ctx context.Context, dir string) error
	watch     func(ctx context.Context, dir string, changed func())
	ask       func(win *mygo.Window, q question) bool
}

func defaultDeps() deps {
	return deps{
		available: git.Available,
		open:      openCLI,
		initRepo:  func(ctx context.Context, dir string) error { return runGit(ctx, dir, "init", "--quiet") },
		watch:     watchDir,
		ask:       askNative,
	}
}

func openCLI(ctx context.Context, dir, dataDir string) (repo, error) {
	r, err := git.Open(ctx, dir)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256([]byte(r.Root))
	parts := append([]string{dataDir}, shelfDataDirs...)
	shelf := git.Shelf{Dir: filepath.Join(append(parts, hex.EncodeToString(sum[:])[:shelfHashLength])...)}
	return &cliRepo{Repo: r, shelf: shelf}, nil
}
