package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/ZerGo0/editor/internal/telemetry"
)

var (
	ErrNotRepo     = errors.New("git: not a git repository")
	ErrNotFound    = errors.New("git: not found")
	ErrNoUpstream  = errors.New("git: no upstream branch")
	ErrUnavailable = errors.New("git: executable not found on PATH")
	ErrEmptyPatch  = errors.New("git: empty patch")
	ErrNoOperation = errors.New("git: no operation in progress")
	ErrDamaged     = errors.New("git: damaged repository")
)

const (
	gitBinary         = "git"
	notRepoMessage    = "not a git repository"
	notWorkTreeMarker = "work tree"
	processWaitDelay  = 3 * time.Second
	openOutputLines   = 2
	progressFlag      = "--progress"
	nulSeparator      = "\x00"
	gitEntry          = ".git"
	xcrunMarker       = "xcrun: error"
)

var (
	baseConfig = []string{"-c", "core.quotepath=false", "-c", "color.ui=never"}
	baseEnv    = []string{
		"LC_ALL=C",
		"GIT_TERMINAL_PROMPT=0",
		"GIT_PAGER=cat",
		"GIT_EDITOR=true",
		"GIT_MERGE_AUTOEDIT=no",
		"GIT_LITERAL_PATHSPECS=1",
	}
	readOnlyEnv = "GIT_OPTIONAL_LOCKS=0"
)

var found atomic.Bool

func Available() bool {
	if found.Load() {
		return true
	}
	_, err := exec.LookPath(gitBinary)
	found.Store(err == nil)
	return err == nil
}

type Error struct {
	Args     []string
	Stderr   string
	ExitCode int
	Dir      string
}

func (e *Error) Error() string {
	name := gitBinary
	if len(e.Args) > 0 {
		name += " " + e.Args[0]
	}
	msg := FirstLine(e.Stderr)
	if msg == "" {
		return fmt.Sprintf("%s: exit status %d", name, e.ExitCode)
	}
	return fmt.Sprintf("%s: %s (exit %d)", name, msg, e.ExitCode)
}

type Repo struct {
	Root   string
	GitDir string
}

type call struct {
	stdin    io.Reader
	env      []string
	progress func(string)
	readOnly bool
}

var readOnly = call{readOnly: true}

func Open(ctx context.Context, dir string) (*Repo, error) {
	out, err := execGit(ctx, dir, readOnly, "rev-parse", "--show-toplevel", "--absolute-git-dir")
	if err != nil {
		var ge *Error
		switch {
		case errors.As(err, &ge) && strings.Contains(ge.Stderr, xcrunMarker):
			return nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
		case errors.As(err, &ge) && strings.Contains(ge.Stderr, notRepoMessage) && hasGitEntry(dir):
			return nil, fmt.Errorf("%w: %w", ErrDamaged, err)
		case errors.As(err, &ge) && (strings.Contains(ge.Stderr, notRepoMessage) || strings.Contains(ge.Stderr, notWorkTreeMarker)):
			return nil, fmt.Errorf("%w: %s", ErrNotRepo, dir)
		}
		return nil, err
	}
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	if len(lines) != openOutputLines || lines[0] == "" || lines[1] == "" {
		return nil, fmt.Errorf("git rev-parse: unexpected output %q", out)
	}
	return &Repo{Root: lines[0], GitDir: lines[1]}, nil
}

func hasGitEntry(dir string) bool {
	for {
		if _, err := os.Lstat(filepath.Join(dir, gitEntry)); err == nil {
			return true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return false
		}
		dir = parent
	}
}

func (r *Repo) read(ctx context.Context, args ...string) ([]byte, error) {
	return execGit(ctx, r.Root, readOnly, args...)
}

func (r *Repo) readText(ctx context.Context, args ...string) (string, error) {
	out, err := r.read(ctx, args...)
	return strings.TrimRight(string(out), "\n"), err
}

func (r *Repo) write(ctx context.Context, args ...string) ([]byte, error) {
	return execGit(ctx, r.Root, call{}, args...)
}

func (r *Repo) run(ctx context.Context, c call, args ...string) ([]byte, error) {
	return execGit(ctx, r.Root, c, args...)
}

func execGit(ctx context.Context, dir string, c call, args ...string) ([]byte, error) {
	if !Available() {
		return nil, ErrUnavailable
	}
	ctx, span := telemetry.Start(ctx, telemetry.SpanGitPrefix+argName(args))
	defer span.End()
	cmd := exec.CommandContext(ctx, gitBinary, append(slices.Clone(baseConfig), args...)...)
	cmd.Dir = dir
	cmd.WaitDelay = processWaitDelay
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM) }
	env := append(os.Environ(), baseEnv...)
	if c.readOnly {
		env = append(env, readOnlyEnv)
	}
	cmd.Env = append(env, c.env...)
	cmd.Stdin = c.stdin
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	var splitter *lineSplitter
	if c.progress != nil {
		splitter = &lineSplitter{emit: c.progress}
		cmd.Stderr = io.MultiWriter(&stderr, splitter)
	}
	err := cmd.Run()
	if splitter != nil {
		splitter.flush()
	}
	if err == nil {
		return stdout.Bytes(), nil
	}
	if ctx.Err() != nil {
		return nil, fmt.Errorf("git %s: %w", argName(args), ctx.Err())
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		return stdout.Bytes(), &Error{Args: args, Stderr: msg, ExitCode: exitErr.ExitCode(), Dir: dir}
	}
	return nil, fmt.Errorf("git %s: %w", argName(args), err)
}

type lineSplitter struct {
	pending []byte
	emit    func(string)
}

func (s *lineSplitter) Write(p []byte) (int, error) {
	s.pending = append(s.pending, p...)
	for {
		i := bytes.IndexAny(s.pending, "\r\n")
		if i < 0 {
			return len(p), nil
		}
		if i > 0 {
			s.emit(string(s.pending[:i]))
		}
		s.pending = s.pending[i+1:]
	}
}

func (s *lineSplitter) flush() {
	if len(s.pending) > 0 {
		s.emit(string(s.pending))
		s.pending = nil
	}
}

type OpResult struct {
	Stopped bool
	Output  string
}

func (r *Repo) finishOp(ctx context.Context, out []byte, err error) (OpResult, error) {
	res := OpResult{Output: strings.TrimSpace(string(out))}
	var ge *Error
	if err != nil {
		if !errors.As(err, &ge) {
			return res, err
		}
		res.Output = strings.TrimSpace(res.Output + "\n" + ge.Stderr)
	}
	st, stErr := r.State(ctx)
	if stErr != nil {
		return res, stErr
	}
	if st.Kind != StateNone {
		res.Stopped = true
		return res, nil
	}
	if err == nil {
		return res, nil
	}
	conflicts, cErr := r.Conflicts(ctx)
	if cErr == nil && len(conflicts) > 0 {
		res.Stopped = true
		return res, nil
	}
	return res, err
}

func (r *Repo) hasHead(ctx context.Context) (bool, error) {
	_, err := r.read(ctx, "rev-parse", "--verify", "--quiet", "HEAD")
	if err == nil {
		return true, nil
	}
	if exitCodeIs(err, 1) {
		return false, nil
	}
	return false, err
}

func exitCodeIs(err error, code int) bool {
	var ge *Error
	return errors.As(err, &ge) && ge.ExitCode == code
}

func argName(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[0]
}

func FirstLine(s string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	return strings.TrimSpace(line)
}

func splitNUL(data []byte) []string {
	if len(data) == 0 {
		return nil
	}
	parts := strings.Split(string(data), nulSeparator)
	if parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	return parts
}

func checkRef(ref string) error {
	if ref == "" || strings.HasPrefix(ref, "-") || strings.ContainsAny(ref, "\x00\n") {
		return fmt.Errorf("git: invalid ref %q", ref)
	}
	return nil
}

func checkRefs(refs ...string) error {
	for _, ref := range refs {
		if err := checkRef(ref); err != nil {
			return err
		}
	}
	return nil
}

func checkPath(p string) error {
	if p == "" || strings.HasPrefix(p, "/") || strings.Contains(p, "\x00") {
		return fmt.Errorf("git: invalid path %q", p)
	}
	return nil
}

func checkLocation(p string) error {
	if p == "" || strings.HasPrefix(p, "-") || strings.Contains(p, "\x00") {
		return fmt.Errorf("git: invalid path %q", p)
	}
	return nil
}

func checkPaths(paths []string) error {
	for _, p := range paths {
		if err := checkPath(p); err != nil {
			return err
		}
	}
	return nil
}

func withPaths(args []string, paths []string) []string {
	return append(append(args, "--"), paths...)
}
