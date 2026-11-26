package git

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"
)

const (
	defaultFileMode = "100644"
	newFileModeLine = "new file mode "
	delFileModeLine = "deleted file mode "
)

func BuildPatch(fd FileDiff, selected func(hunk, line int) bool, reverse bool) []byte {
	if fd.Binary || selected == nil {
		return nil
	}
	var hunks []Hunk
	delta, total, picked := 0, 0, 0
	for hi, h := range fd.Hunks {
		out := Hunk{Header: h.Header}
		changed := false
		for li, l := range h.Lines {
			if l.Kind == ' ' {
				out.Lines = append(out.Lines, l)
				continue
			}
			total++
			if selected(hi, li) {
				picked++
				changed = true
				out.Lines = append(out.Lines, l)
				continue
			}
			absent := byte('+')
			if reverse {
				absent = '-'
			}
			if l.Kind != absent {
				l.Kind = ' '
				out.Lines = append(out.Lines, l)
			}
		}
		if !changed {
			continue
		}
		for _, l := range out.Lines {
			if l.Kind != '+' {
				out.OldLines++
			}
			if l.Kind != '-' {
				out.NewLines++
			}
		}
		if reverse {
			first := firstLine1(h.NewStart, h.NewLines)
			out.NewStart = h.NewStart
			out.OldStart = rangeStart(first-delta, out.OldLines)
		} else {
			first := firstLine1(h.OldStart, h.OldLines)
			out.OldStart = h.OldStart
			out.NewStart = rangeStart(first+delta, out.NewLines)
		}
		delta += out.NewLines - out.OldLines
		hunks = append(hunks, out)
	}
	if picked == 0 {
		return nil
	}
	var buf bytes.Buffer
	writePatchHeader(&buf, fd, reverse, picked == total)
	for _, h := range hunks {
		writeHunk(&buf, h)
	}
	return buf.Bytes()
}

func firstLine1(start, lines int) int {
	if lines == 0 {
		return start + 1
	}
	return start
}

func rangeStart(first, lines int) int {
	if lines == 0 {
		return first - 1
	}
	return first
}

func writePatchHeader(buf *bytes.Buffer, fd FileDiff, reverse, all bool) {
	oldPath, newPath := fd.OldPath, fd.NewPath
	isNew, isDeleted := fd.IsNew(), fd.IsDeleted()
	if isNew && reverse && !all {
		oldPath, isNew = newPath, false
	}
	if isDeleted && !reverse && !all {
		newPath, isDeleted = oldPath, false
	}
	nameA, nameB := oldPath, newPath
	if isNew {
		nameA = newPath
	}
	if isDeleted {
		nameB = oldPath
	}
	fmt.Fprintf(buf, "diff --git %s %s\n", quotePath(oldPrefix+nameA), quotePath(newPrefix+nameB))
	switch {
	case isNew:
		buf.WriteString(modeLine(fd, newFileModeLine) + "\n")
	case isDeleted:
		buf.WriteString(modeLine(fd, delFileModeLine) + "\n")
	case fd.IsRename():
		for _, h := range fd.Header {
			if hasAnyPrefix(h, "rename from ", "rename to ", "similarity index ") {
				buf.WriteString(h + "\n")
			}
		}
	}
	if isNew {
		buf.WriteString("--- " + devNull + "\n")
	} else {
		buf.WriteString("--- " + quotePath(oldPrefix+oldPath) + "\n")
	}
	if isDeleted {
		buf.WriteString("+++ " + devNull + "\n")
	} else {
		buf.WriteString("+++ " + quotePath(newPrefix+newPath) + "\n")
	}
}

func modeLine(fd FileDiff, prefix string) string {
	for _, h := range fd.Header {
		if strings.HasPrefix(h, prefix) {
			return h
		}
	}
	return prefix + defaultFileMode
}

func hasAnyPrefix(s string, prefixes ...string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

func writeHunk(buf *bytes.Buffer, h Hunk) {
	fmt.Fprintf(buf, "@@ -%s +%s @@", hunkRange(h.OldStart, h.OldLines), hunkRange(h.NewStart, h.NewLines))
	if h.Header != "" {
		buf.WriteString(" " + h.Header)
	}
	buf.WriteByte('\n')
	for _, l := range h.Lines {
		buf.WriteByte(l.Kind)
		buf.WriteString(l.Text)
		buf.WriteByte('\n')
		if l.NoEOL {
			buf.WriteString(noEOLText + "\n")
		}
	}
}

func hunkRange(start, lines int) string {
	if lines == 1 {
		return strconv.Itoa(start)
	}
	return strconv.Itoa(start) + "," + strconv.Itoa(lines)
}

func quotePath(p string) string {
	if !strings.ContainsFunc(p, needsQuote) {
		return p
	}
	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(p); i++ {
		c := p[i]
		switch {
		case c == '"' || c == '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
		case c == '\n':
			b.WriteString("\\n")
		case c == '\t':
			b.WriteString("\\t")
		case c == '\r':
			b.WriteString("\\r")
		case c < ' ' || c == 0x7f:
			fmt.Fprintf(&b, "\\%03o", c)
		default:
			b.WriteByte(c)
		}
	}
	b.WriteByte('"')
	return b.String()
}

func needsQuote(c rune) bool {
	return c == '"' || c == '\\' || c < ' ' || c == 0x7f
}

func (r *Repo) Apply(ctx context.Context, patch []byte, cached, reverse bool) error {
	if len(patch) == 0 {
		return ErrEmptyPatch
	}
	args := []string{"apply", "--recount", "--whitespace=nowarn"}
	if cached {
		args = append(args, "--cached")
	}
	if reverse {
		args = append(args, "-R")
	}
	_, err := r.run(ctx, call{stdin: bytes.NewReader(patch)}, append(args, "-")...)
	return err
}

func (r *Repo) StageFiles(ctx context.Context, paths ...string) error {
	if len(paths) == 0 {
		return nil
	}
	if err := checkPaths(paths); err != nil {
		return err
	}
	_, err := r.write(ctx, withPaths([]string{"add", "--all"}, paths)...)
	return err
}

func (r *Repo) UnstageFiles(ctx context.Context, paths ...string) error {
	if len(paths) == 0 {
		return nil
	}
	if err := checkPaths(paths); err != nil {
		return err
	}
	born, err := r.hasHead(ctx)
	if err != nil {
		return err
	}
	if born {
		_, err = r.write(ctx, withPaths([]string{"restore", "--staged"}, paths)...)
		return err
	}
	_, err = r.write(ctx, withPaths([]string{"rm", "--cached", "-r", "--quiet", "--ignore-unmatch"}, paths)...)
	return err
}

func (r *Repo) Discard(ctx context.Context, paths ...string) error {
	if len(paths) == 0 {
		return nil
	}
	if err := checkPaths(paths); err != nil {
		return err
	}
	_, err := r.write(ctx, withPaths([]string{"restore", "--worktree"}, paths)...)
	return err
}

func (r *Repo) DiscardUntracked(ctx context.Context, paths ...string) error {
	if len(paths) == 0 {
		return nil
	}
	if err := checkPaths(paths); err != nil {
		return err
	}
	_, err := r.write(ctx, withPaths([]string{"clean", "--force", "-d"}, paths)...)
	return err
}
