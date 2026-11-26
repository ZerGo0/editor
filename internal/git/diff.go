package git

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const (
	devNull           = "/dev/null"
	oldPrefix         = "a/"
	newPrefix         = "b/"
	diffGitPrefix     = "diff --git "
	diffCombinedA     = "diff --cc "
	diffCombinedB     = "diff --combined "
	hunkMarker        = "@@ "
	combinedMarker    = "@@@"
	noEOLMarker       = "\\"
	noEOLText         = "\\ No newline at end of file"
	binaryFilesPrefix = "Binary files "
	binaryPatch       = "GIT binary patch"
	headerSplitExtra  = len(oldPrefix) + 1 + len(newPrefix)
)

var diffFlags = []string{
	"--no-color", "--no-ext-diff", "--no-textconv", "-M", "-U3",
	"--src-prefix=" + oldPrefix, "--dst-prefix=" + newPrefix, "--submodule=short",
	"--histogram", "--indent-heuristic",
}

var hunkHeaderPattern = regexp.MustCompile("^@@ -(\\d+)(?:,(\\d+))? \\+(\\d+)(?:,(\\d+))? @@(?: (.*))?$")

type Line struct {
	Kind  byte
	Text  string
	OldNo int
	NewNo int
	NoEOL bool
}

type Hunk struct {
	OldStart int
	OldLines int
	NewStart int
	NewLines int
	Header   string
	Lines    []Line
}

type FileDiff struct {
	OldPath string
	NewPath string
	Binary  bool
	Header  []string
	Hunks   []Hunk
}

func (f FileDiff) IsNew() bool     { return f.OldPath == "" }
func (f FileDiff) IsDeleted() bool { return f.NewPath == "" }
func (f FileDiff) IsRename() bool {
	return f.OldPath != "" && f.NewPath != "" && f.OldPath != f.NewPath
}

func (f FileDiff) Path() string {
	if f.NewPath != "" {
		return f.NewPath
	}
	return f.OldPath
}

func (r *Repo) diff(ctx context.Context, extra []string, paths []string) ([]FileDiff, error) {
	args := append(append([]string{"diff"}, diffFlags...), extra...)
	out, err := r.read(ctx, withPaths(args, paths)...)
	if err != nil {
		return nil, err
	}
	return parseDiff(out)
}

func (r *Repo) DiffWorkTree(ctx context.Context, paths ...string) ([]FileDiff, error) {
	return r.diff(ctx, nil, paths)
}

func (r *Repo) DiffStaged(ctx context.Context, paths ...string) ([]FileDiff, error) {
	return r.diff(ctx, []string{"--cached"}, paths)
}

func (r *Repo) DiffCommits(ctx context.Context, a, b string, paths ...string) ([]FileDiff, error) {
	if err := checkRefs(a, b); err != nil {
		return nil, err
	}
	return r.diff(ctx, []string{a, b}, paths)
}

func (r *Repo) DiffCommit(ctx context.Context, rev string) ([]FileDiff, error) {
	if err := checkRef(rev); err != nil {
		return nil, err
	}
	base, err := r.diffBase(ctx, rev)
	if err != nil {
		return nil, err
	}
	return r.diff(ctx, []string{base, rev}, nil)
}

func (r *Repo) diffBase(ctx context.Context, rev string) (string, error) {
	out, err := r.readText(ctx, "rev-list", "--parents", "-n", "1", rev)
	if err != nil {
		return "", err
	}
	fields := strings.Fields(out)
	if len(fields) == 0 {
		return "", fmt.Errorf("git rev-list: no commit for %q", rev)
	}
	if len(fields) > 1 {
		return fields[1], nil
	}
	return r.emptyTree(ctx)
}

func (r *Repo) emptyTree(ctx context.Context) (string, error) {
	out, err := r.run(ctx, call{readOnly: true, stdin: strings.NewReader("")}, "hash-object", "-t", "tree", "--stdin")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func parseDiff(data []byte) ([]FileDiff, error) {
	if len(data) == 0 {
		return nil, nil
	}
	lines := strings.Split(string(data), "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	var files []FileDiff
	cur := -1
	skipping := false
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		switch {
		case strings.HasPrefix(line, diffGitPrefix):
			skipping = false
			files = append(files, parseDiffGitLine(strings.TrimPrefix(line, diffGitPrefix)))
			cur = len(files) - 1
		case strings.HasPrefix(line, diffCombinedA) || strings.HasPrefix(line, diffCombinedB):
			path := strings.TrimPrefix(strings.TrimPrefix(line, diffCombinedA), diffCombinedB)
			files = append(files, FileDiff{OldPath: unquotePath(path), NewPath: unquotePath(path)})
			cur = len(files) - 1
			skipping = true
		case skipping:
		case strings.HasPrefix(line, combinedMarker):
			skipping = true
		case strings.HasPrefix(line, hunkMarker):
			if cur < 0 {
				return nil, fmt.Errorf("git diff: hunk before file header at line %d", i+1)
			}
			h, next, err := parseHunk(lines, i)
			if err != nil {
				return nil, err
			}
			files[cur].Hunks = append(files[cur].Hunks, h)
			i = next
		case cur < 0:
		default:
			applyHeaderLine(&files[cur], line)
		}
	}
	return files, nil
}

func applyHeaderLine(f *FileDiff, line string) {
	switch {
	case strings.HasPrefix(line, "--- "):
		f.OldPath = headerPath(strings.TrimPrefix(line, "--- "), oldPrefix)
	case strings.HasPrefix(line, "+++ "):
		f.NewPath = headerPath(strings.TrimPrefix(line, "+++ "), newPrefix)
	case strings.HasPrefix(line, "rename from "):
		f.OldPath = unquotePath(strings.TrimPrefix(line, "rename from "))
		f.Header = append(f.Header, line)
	case strings.HasPrefix(line, "rename to "):
		f.NewPath = unquotePath(strings.TrimPrefix(line, "rename to "))
		f.Header = append(f.Header, line)
	case strings.HasPrefix(line, "new file mode"):
		f.OldPath = ""
		f.Header = append(f.Header, line)
	case strings.HasPrefix(line, "deleted file mode"):
		f.NewPath = ""
		f.Header = append(f.Header, line)
	case strings.HasPrefix(line, binaryFilesPrefix), line == binaryPatch:
		f.Binary = true
	default:
		f.Header = append(f.Header, line)
	}
}

func headerPath(s, prefix string) string {
	name, _, _ := strings.Cut(s, "\t")
	if name == devNull {
		return ""
	}
	return strings.TrimPrefix(unquotePath(name), prefix)
}

func parseDiffGitLine(rest string) FileDiff {
	var f FileDiff
	oldName, newName, ok := splitDiffGitNames(rest)
	if ok {
		f.OldPath = strings.TrimPrefix(oldName, oldPrefix)
		f.NewPath = strings.TrimPrefix(newName, newPrefix)
	}
	return f
}

func splitDiffGitNames(rest string) (string, string, bool) {
	if strings.HasPrefix(rest, "\"") {
		first, remainder, ok := readQuoted(rest)
		if !ok {
			return "", "", false
		}
		second := strings.TrimPrefix(remainder, " ")
		if strings.HasPrefix(second, "\"") {
			second, _, ok = readQuoted(second)
			if !ok {
				return "", "", false
			}
		}
		return first, second, true
	}
	if idx := strings.Index(rest, " \""); idx >= 0 {
		second, _, ok := readQuoted(rest[idx+1:])
		return rest[:idx], second, ok
	}
	n := len(rest) - headerSplitExtra
	if n < 0 || n%2 != 0 {
		return "", "", false
	}
	n /= 2
	split := len(oldPrefix) + n
	left, right := rest[:split], rest[split+1:]
	if rest[split] != ' ' || !strings.HasPrefix(left, oldPrefix) || !strings.HasPrefix(right, newPrefix) {
		return "", "", false
	}
	if strings.TrimPrefix(left, oldPrefix) != strings.TrimPrefix(right, newPrefix) {
		return "", "", false
	}
	return left, right, true
}

func readQuoted(s string) (string, string, bool) {
	for i := 1; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
		case '"':
			name, err := strconv.Unquote(s[:i+1])
			if err != nil {
				return "", "", false
			}
			return name, s[i+1:], true
		}
	}
	return "", "", false
}

func unquotePath(s string) string {
	if strings.HasPrefix(s, "\"") {
		if name, rest, ok := readQuoted(s); ok && rest == "" {
			return name
		}
	}
	return s
}

func parseHunk(lines []string, start int) (Hunk, int, error) {
	m := hunkHeaderPattern.FindStringSubmatch(lines[start])
	if m == nil {
		return Hunk{}, 0, fmt.Errorf("git diff: malformed hunk header %q", lines[start])
	}
	h := Hunk{Header: m[5]}
	h.OldStart, h.OldLines = atoiDefault(m[1], 0), atoiDefault(m[2], 1)
	h.NewStart, h.NewLines = atoiDefault(m[3], 0), atoiDefault(m[4], 1)
	oldNo, newNo := h.OldStart, h.NewStart
	if h.OldLines == 0 {
		oldNo++
	}
	if h.NewLines == 0 {
		newNo++
	}
	remOld, remNew := h.OldLines, h.NewLines
	i := start
	for remOld > 0 || remNew > 0 {
		i++
		if i >= len(lines) {
			return Hunk{}, 0, fmt.Errorf("git diff: truncated hunk %q", lines[start])
		}
		text := lines[i]
		kind := byte(' ')
		if text != "" {
			kind = text[0]
			text = text[1:]
		}
		switch kind {
		case ' ':
			if remOld == 0 || remNew == 0 {
				return Hunk{}, 0, fmt.Errorf("git diff: hunk %q has too many lines", lines[start])
			}
			h.Lines = append(h.Lines, Line{Kind: ' ', Text: text, OldNo: oldNo, NewNo: newNo})
			oldNo, newNo, remOld, remNew = oldNo+1, newNo+1, remOld-1, remNew-1
		case '-':
			if remOld == 0 {
				return Hunk{}, 0, fmt.Errorf("git diff: hunk %q has too many removed lines", lines[start])
			}
			h.Lines = append(h.Lines, Line{Kind: '-', Text: text, OldNo: oldNo})
			oldNo, remOld = oldNo+1, remOld-1
		case '+':
			if remNew == 0 {
				return Hunk{}, 0, fmt.Errorf("git diff: hunk %q has too many added lines", lines[start])
			}
			h.Lines = append(h.Lines, Line{Kind: '+', Text: text, NewNo: newNo})
			newNo, remNew = newNo+1, remNew-1
		case noEOLMarker[0]:
			if len(h.Lines) == 0 {
				return Hunk{}, 0, fmt.Errorf("git diff: unexpected %q", lines[i])
			}
			h.Lines[len(h.Lines)-1].NoEOL = true
		default:
			return Hunk{}, 0, fmt.Errorf("git diff: unexpected line %q in hunk", lines[i])
		}
	}
	for i+1 < len(lines) && strings.HasPrefix(lines[i+1], noEOLMarker) {
		i++
		if len(h.Lines) > 0 {
			h.Lines[len(h.Lines)-1].NoEOL = true
		}
	}
	return h, i, nil
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}
