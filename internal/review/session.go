package review

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/ZerGo0/editor/internal/fsutil"
)

const (
	DirName     = "reviews"
	requestName = "request.json"
	resultName  = "result.json"
	dirPerm     = 0o700
	filePerm    = 0o600
	staleAge    = time.Minute
)

var ErrAgentGone = errors.New("the agent stopped waiting for this review")

type Kind string

const (
	KindReview   Kind = "review"
	KindAnnotate Kind = "annotate"
	KindPlan     Kind = "plan"
)

type PRRef struct {
	Host   string `json:"host"`
	Owner  string `json:"owner"`
	Repo   string `json:"repo"`
	Number int    `json:"number"`
	URL    string `json:"url"`
}

type Request struct {
	Kind   Kind     `json:"kind"`
	Folder string   `json:"folder"`
	PID    int      `json:"pid"`
	Diff   DiffMode `json:"diff,omitempty"`
	Base   string   `json:"base,omitempty"`
	PR     *PRRef   `json:"pr,omitempty"`
	File   string   `json:"file,omitempty"`
	Text   string   `json:"text,omitempty"`
	Gate   bool     `json:"gate,omitempty"`
	Notes  bool     `json:"notes,omitempty"`
}

func (r Request) CanApprove() bool { return r.Kind != KindAnnotate || r.Gate }

func (r Request) Validate() error {
	switch {
	case !filepath.IsAbs(r.Folder):
		return fmt.Errorf("review request: folder %q is not absolute", r.Folder)
	case r.Kind == KindReview && r.PR == nil && !r.Diff.Valid():
		return fmt.Errorf("review request: unknown diff %q", r.Diff)
	case r.Kind == KindReview && r.PR != nil && (r.PR.Owner == "" || r.PR.Repo == "" || r.PR.Number <= 0):
		return errors.New("review request: incomplete pull request")
	case r.Kind == KindAnnotate && !filepath.IsAbs(r.File):
		return fmt.Errorf("review request: file %q is not absolute", r.File)
	case r.Kind == KindPlan && r.Text == "":
		return errors.New("review request: the plan is empty")
	case r.Kind != KindReview && r.Kind != KindAnnotate && r.Kind != KindPlan:
		return fmt.Errorf("review request: unknown kind %q", r.Kind)
	}
	return nil
}

type Decision string

const (
	Approved  Decision = "approved"
	Annotated Decision = "annotated"
	Dismissed Decision = "dismissed"
)

type Result struct {
	Decision    Decision     `json:"decision"`
	Annotations []Annotation `json:"annotations,omitempty"`
	Diff        DiffMode     `json:"diff,omitempty"`
	Base        string       `json:"base,omitempty"`
	PR          *PR          `json:"pr,omitempty"`
}

type Session struct {
	Dir string
}

func NewSession(userData string) (Session, error) {
	root := filepath.Join(userData, DirName)
	if err := os.MkdirAll(root, dirPerm); err != nil {
		return Session{}, fmt.Errorf("review session: %w", err)
	}
	pruneSessions(root, time.Now())
	dir, err := os.MkdirTemp(root, "")
	if err != nil {
		return Session{}, fmt.Errorf("review session: %w", err)
	}
	return Session{Dir: dir}, nil
}

func pruneSessions(root string, now time.Time) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		s := Session{Dir: filepath.Join(root, e.Name())}
		req, err := readJSON[Request](s.RequestPath())
		if err == nil && Alive(req.PID) {
			continue
		}
		if info, statErr := e.Info(); err != nil && statErr == nil && now.Sub(info.ModTime()) < staleAge {
			continue
		}
		_ = s.Remove()
	}
}

func (s Session) RequestPath() string { return filepath.Join(s.Dir, requestName) }

func (s Session) resultPath() string { return filepath.Join(s.Dir, resultName) }

func (s Session) Write(r Request) error {
	return writeJSON(s.RequestPath(), r)
}

func OpenRequest(userData, path string) (Session, Request, error) {
	root := filepath.Join(userData, DirName)
	path = filepath.Clean(path)
	if filepath.Base(path) != requestName || !fsutil.Within(root, path) {
		return Session{}, Request{}, fmt.Errorf("review request: %q is not a review session file", path)
	}
	req, err := readJSON[Request](path)
	if err != nil {
		return Session{}, Request{}, err
	}
	return Session{Dir: filepath.Dir(path)}, req, req.Validate()
}

func (s Session) Decide(req Request, res Result) error {
	if !Alive(req.PID) {
		return ErrAgentGone
	}
	err := writeJSON(s.resultPath(), res)
	if errors.Is(err, fs.ErrNotExist) {
		return ErrAgentGone
	}
	return err
}

func (s Session) Wait(ctx context.Context, poll time.Duration) (Result, error) {
	t := time.NewTicker(poll)
	defer t.Stop()
	for {
		res, err := readJSON[Result](s.resultPath())
		if err == nil {
			return res, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return Result{}, err
		}
		select {
		case <-ctx.Done():
			return Result{}, ctx.Err()
		case <-t.C:
		}
	}
}

func (s Session) Remove() error { return os.RemoveAll(s.Dir) }

func Alive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

func writeJSON(path string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(path, data, filePerm)
}

func readJSON[T any](path string) (T, error) {
	var v T
	data, err := os.ReadFile(path)
	if err != nil {
		return v, err
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return v, fmt.Errorf("read %s: %w", filepath.Base(path), err)
	}
	return v, nil
}
