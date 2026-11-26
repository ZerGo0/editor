package git

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

const (
	lineLogRecordStart = "\x1e"
	lineLogRecordEnd   = "\x1f"
	lineLogFormat      = "%x1e" + commitFormat + "%x1f"
)

type LineLogEntry struct {
	Commit Commit
	Diffs  []FileDiff
}

func (r *Repo) LineLog(ctx context.Context, path string, start, end int) ([]LineLogEntry, error) {
	if err := checkPath(path); err != nil {
		return nil, err
	}
	if start < 1 || end < start {
		return nil, fmt.Errorf("git log: invalid line range %d,%d", start, end)
	}
	args := append([]string{"log", "--no-show-signature", "--encoding=UTF-8", "--format=" + lineLogFormat}, diffFlags...)
	args = append(args, "-L", fmt.Sprintf("%d,%d:%s", start, end, path))
	out, err := r.read(ctx, args...)
	if err != nil {
		var ge *Error
		if errors.As(err, &ge) && strings.Contains(ge.Stderr, noCommitsMarker) {
			return nil, nil
		}
		return nil, err
	}
	return parseLineLog(string(out))
}

func parseLineLog(text string) ([]LineLogEntry, error) {
	var entries []LineLogEntry
	for text != "" {
		rest, ok := strings.CutPrefix(text, lineLogRecordStart)
		if !ok {
			return nil, errors.New("git log -L: unexpected output")
		}
		header, rest, ok := strings.Cut(rest, lineLogRecordEnd)
		if !ok {
			return nil, errors.New("git log -L: unterminated commit header")
		}
		commit, err := parseCommitFields(strings.Split(header, nulSeparator))
		if err != nil {
			return nil, err
		}
		patch := rest
		text = ""
		if i := strings.Index(rest, "\n"+lineLogRecordStart); i >= 0 {
			patch, text = rest[:i], rest[i+1:]
		}
		diffs, err := parseDiff([]byte(patch))
		if err != nil {
			return nil, err
		}
		entries = append(entries, LineLogEntry{Commit: commit, Diffs: diffs})
	}
	return entries, nil
}
