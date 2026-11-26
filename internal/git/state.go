package git

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type StateKind int

const (
	StateNone StateKind = iota
	StateMerging
	StateRebasing
	StateRebaseInteractive
	StateCherryPicking
	StateReverting
	StateBisecting
)

const (
	rebaseMergeDir   = "rebase-merge"
	rebaseApplyDir   = "rebase-apply"
	sequencerTodo    = "sequencer/todo"
	detachedHeadName = "detached HEAD"
	todoCommentMark  = "#"
	todoRevertVerb   = "revert"
)

var stateNames = map[StateKind]string{
	StateNone:              "none",
	StateMerging:           "merging",
	StateRebasing:          "rebasing",
	StateRebaseInteractive: "interactive rebase",
	StateCherryPicking:     "cherry-picking",
	StateReverting:         "reverting",
	StateBisecting:         "bisecting",
}

func (k StateKind) String() string { return stateNames[k] }

type RepoState struct {
	Kind  StateKind
	Head  string
	Onto  string
	Step  int
	Total int
}

func (r *Repo) gitPath(parts ...string) string {
	return filepath.Join(append([]string{r.GitDir}, parts...)...)
}

func (r *Repo) gitFile(parts ...string) (string, bool, error) {
	data, err := os.ReadFile(r.gitPath(parts...))
	if errors.Is(err, fs.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return strings.TrimSpace(string(data)), true, nil
}

func (r *Repo) gitDirExists(name string) bool {
	info, err := os.Stat(r.gitPath(name))
	return err == nil && info.IsDir()
}

func (r *Repo) State(ctx context.Context) (RepoState, error) {
	if err := ctx.Err(); err != nil {
		return RepoState{}, err
	}
	if r.gitDirExists(rebaseMergeDir) {
		return r.rebaseMergeState()
	}
	if r.gitDirExists(rebaseApplyDir) {
		if _, ok, err := r.gitFile(rebaseApplyDir, "rebasing"); err != nil {
			return RepoState{}, err
		} else if ok {
			return r.rebaseApplyState()
		}
	}
	for _, probe := range []struct {
		file string
		kind StateKind
	}{
		{"REVERT_HEAD", StateReverting},
		{"CHERRY_PICK_HEAD", StateCherryPicking},
		{"MERGE_HEAD", StateMerging},
	} {
		head, ok, err := r.gitFile(probe.file)
		if err != nil {
			return RepoState{}, err
		}
		if ok {
			return RepoState{Kind: probe.kind, Head: firstLine(head)}, nil
		}
	}
	if todo, ok, err := r.gitFile(sequencerTodo); err != nil {
		return RepoState{}, err
	} else if ok {
		if kind := sequencerKind(todo); kind != StateNone {
			return RepoState{Kind: kind}, nil
		}
	}
	if _, ok, err := r.gitFile("BISECT_LOG"); err != nil {
		return RepoState{}, err
	} else if ok {
		return RepoState{Kind: StateBisecting}, nil
	}
	return RepoState{}, nil
}

func sequencerKind(todo string) StateKind {
	for _, line := range strings.Split(todo, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, todoCommentMark) {
			continue
		}
		verb, _, _ := strings.Cut(line, " ")
		switch verb {
		case todoRevertVerb:
			return StateReverting
		default:
			return StateCherryPicking
		}
	}
	return StateNone
}

func (r *Repo) rebaseMergeState() (RepoState, error) {
	st := RepoState{Kind: StateRebasing}
	if _, ok, err := r.gitFile(rebaseMergeDir, "interactive"); err != nil {
		return st, err
	} else if ok {
		st.Kind = StateRebaseInteractive
	}
	if err := r.readRebaseRefs(&st, rebaseMergeDir); err != nil {
		return st, err
	}
	step, haveStep, err := r.gitInt(rebaseMergeDir, "msgnum")
	if err != nil {
		return st, err
	}
	total, haveTotal, err := r.gitInt(rebaseMergeDir, "end")
	if err != nil {
		return st, err
	}
	if haveStep && haveTotal {
		st.Step, st.Total = step, total
		return st, nil
	}
	done, _, err := r.gitFile(rebaseMergeDir, "done")
	if err != nil {
		return st, err
	}
	todo, _, err := r.gitFile(rebaseMergeDir, "git-rebase-todo")
	if err != nil {
		return st, err
	}
	st.Step = countTodoLines(done)
	st.Total = st.Step + countTodoLines(todo)
	return st, nil
}

func (r *Repo) rebaseApplyState() (RepoState, error) {
	st := RepoState{Kind: StateRebasing}
	if err := r.readRebaseRefs(&st, rebaseApplyDir); err != nil {
		return st, err
	}
	var err error
	if st.Step, _, err = r.gitInt(rebaseApplyDir, "next"); err != nil {
		return st, err
	}
	st.Total, _, err = r.gitInt(rebaseApplyDir, "last")
	return st, err
}

func (r *Repo) readRebaseRefs(st *RepoState, dir string) error {
	head, _, err := r.gitFile(dir, "head-name")
	if err != nil {
		return err
	}
	if head != detachedHeadName {
		st.Head = strings.TrimPrefix(head, headRefPrefix)
	}
	st.Onto, _, err = r.gitFile(dir, "onto")
	return err
}

func (r *Repo) gitInt(parts ...string) (int, bool, error) {
	text, ok, err := r.gitFile(parts...)
	if err != nil || !ok {
		return 0, false, err
	}
	n, err := strconv.Atoi(text)
	if err != nil {
		return 0, false, fmt.Errorf("git: malformed %s: %q", filepath.Join(parts...), text)
	}
	return n, true, nil
}

func countTodoLines(text string) int {
	n := 0
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, todoCommentMark) {
			n++
		}
	}
	return n
}

func (k StateKind) command() (string, error) {
	switch k {
	case StateMerging:
		return "merge", nil
	case StateRebasing, StateRebaseInteractive:
		return "rebase", nil
	case StateCherryPicking:
		return "cherry-pick", nil
	case StateReverting:
		return "revert", nil
	case StateNone:
		return "", ErrNoOperation
	}
	return "", fmt.Errorf("git: cannot continue or skip while %s", k)
}

func (r *Repo) Continue(ctx context.Context) (OpResult, error) {
	return r.advance(ctx, "--continue")
}

func (r *Repo) Skip(ctx context.Context) (OpResult, error) {
	st, err := r.State(ctx)
	if err != nil {
		return OpResult{}, err
	}
	if st.Kind == StateMerging {
		return OpResult{}, errors.New("git: a merge cannot be skipped")
	}
	return r.advance(ctx, "--skip")
}

func (r *Repo) advance(ctx context.Context, flag string) (OpResult, error) {
	st, err := r.State(ctx)
	if err != nil {
		return OpResult{}, err
	}
	verb, err := st.Kind.command()
	if err != nil {
		return OpResult{}, err
	}
	out, err := r.write(ctx, verb, flag)
	res, err := r.finishOp(ctx, out, err)
	if err == nil && !res.Stopped {
		r.removeRebaseHelpers()
	}
	return res, err
}

func (r *Repo) Abort(ctx context.Context) error {
	st, err := r.State(ctx)
	if err != nil {
		return err
	}
	if st.Kind == StateBisecting {
		_, err = r.write(ctx, "bisect", "reset")
		return err
	}
	verb, err := st.Kind.command()
	if err != nil {
		return err
	}
	if _, err = r.write(ctx, verb, "--abort"); err != nil {
		return err
	}
	r.removeRebaseHelpers()
	return nil
}
