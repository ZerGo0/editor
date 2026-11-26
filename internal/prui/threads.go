package prui

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"unicode/utf8"

	"github.com/ZerGo0/editor/internal/fsutil"
	"github.com/ZerGo0/editor/internal/github"
	"github.com/egoist/mygo/ui/editor"
)

const (
	annotationCharsPerLine = 66
	annotationLineHeight   = 18
	annotationCommentHead  = 28
	annotationCommentPad   = 4
	annotationChrome       = 52
	annotationTagRow       = 26
	annotationCollapsed    = 40
)

var ErrBinaryFile = errors.New("prui: binary file, no text diff to show")

func threadsForPath(threads []github.Thread, path string) []github.Thread {
	var out []github.Thread
	for _, t := range threads {
		if t.Path == path {
			out = append(out, t)
		}
	}
	return out
}

func threadSide(t github.Thread) editor.DiffSide {
	if t.Side == github.SideLeft {
		return editor.DiffOld
	}
	return editor.DiffNew
}

func wrappedLines(text string) int {
	lines := 0
	for _, line := range bytes.Split([]byte(text), []byte("\n")) {
		n := utf8.RuneCount(line)
		lines += max(1, (n+annotationCharsPerLine-1)/annotationCharsPerLine)
	}
	return lines
}

func threadHeight(t github.Thread, expanded bool) float32 {
	if t.Resolved && !expanded {
		return annotationCollapsed
	}
	h := float32(annotationChrome)
	if t.Resolved || t.Outdated {
		h += annotationTagRow
	}
	for _, c := range t.Comments {
		h += annotationCommentHead + annotationCommentPad + float32(wrappedLines(c.Body)*annotationLineHeight)
	}
	return h
}

func buildAnnotations(threads []github.Thread, path string, expanded map[string]bool, oldLines, newLines int) []editor.DiffAnnotation {
	var out []editor.DiffAnnotation
	for _, t := range threadsForPath(threads, path) {
		side := threadSide(t)
		count := newLines
		if side == editor.DiffOld {
			count = oldLines
		}
		line := min(max(t.Line-1, 0), max(count-1, 0))
		out = append(out, editor.DiffAnnotation{Side: side, Line: line, Height: threadHeight(t, expanded[t.ID]), Key: t.ID})
	}
	return out
}

func lineComment(r editor.DiffLineRange, path, body string) github.ReviewCommentOptions {
	side := github.SideRight
	if r.Side == editor.DiffOld {
		side = github.SideLeft
	}
	opts := github.ReviewCommentOptions{Body: body, Path: path, Line: r.End + 1, Side: side}
	if r.Start < r.End {
		opts.StartLine = r.Start + 1
	}
	return opts
}

type fileSource struct {
	local Local
	be    Backend
	repo  github.Remote
	root  string
}

func (s fileSource) content(ctx context.Context, ref, path string) (string, error) {
	var data []byte
	var err error
	if s.local != nil {
		data, err = s.local.Show(ctx, ref, path)
	}
	if s.local == nil || err != nil {
		if data, err = s.be.FileAt(ctx, s.repo, ref, path); err != nil {
			return "", err
		}
	}
	if bytes.IndexByte(data, 0) >= 0 || !utf8.Valid(data) {
		return "", ErrBinaryFile
	}
	return string(data), nil
}

type fileVersions struct {
	old, new string
	baseSHA  string
}

func (s fileSource) working(path string) (string, error) {
	target := filepath.Join(s.root, filepath.FromSlash(path))
	if s.root == "" || !fsutil.Within(s.root, target) {
		return "", fmt.Errorf("prui: %q is outside the project folder", path)
	}
	data, err := os.ReadFile(target)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "", nil
	case err != nil:
		return "", err
	case bytes.IndexByte(data, 0) >= 0 || !utf8.Valid(data):
		return "", ErrBinaryFile
	}
	return string(data), nil
}

func loadVersions(ctx context.Context, src fileSource, pr github.PullRequest, f github.PRFile, spec diffSpec, baseSHA string) (fileVersions, error) {
	v := fileVersions{baseSHA: baseSHA}
	oldPath := cmp.Or(f.PrevPath, f.Path)
	switch spec.mode {
	case modeLocal:
		v.baseSHA = pr.HeadSHA
		if f.Status != statusRemoved {
			text, err := src.working(f.Path)
			if err != nil {
				return v, err
			}
			v.new = text
		}
		if f.Status != statusAdded {
			text, err := src.content(ctx, pr.HeadSHA, f.Path)
			if err != nil {
				return v, err
			}
			v.old = text
		}
		return v, nil
	case modeSince:
		v.baseSHA = spec.oldRef
	}
	if f.Status != statusRemoved {
		text, err := src.content(ctx, pr.HeadSHA, f.Path)
		if err != nil {
			return v, err
		}
		v.new = text
	}
	if f.Status == statusAdded {
		return v, nil
	}
	if v.baseSHA == "" {
		sha, err := src.be.MergeBase(ctx, src.repo, pr.BaseRef, pr.HeadSHA)
		if err != nil {
			sha = pr.BaseRef
		}
		v.baseSHA = sha
	}
	text, err := src.content(ctx, v.baseSHA, oldPath)
	if err != nil {
		return v, err
	}
	v.old = text
	return v, nil
}
