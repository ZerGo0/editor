package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	blameHeaderMinFields = 3
	blameHeaderMaxFields = 4
	minOIDLength         = 40
	tzLength             = 5
	secondsPerHour       = 3600
	secondsPerMinute     = 60
	blameNoSuchPath      = "no such path"
)

type BlameLine struct {
	Line     int
	OID      string
	Author   string
	Time     time.Time
	Summary  string
	OrigLine int
	Text     string
	Filename string
}

func (b BlameLine) IsUncommitted() bool {
	return b.OID != "" && strings.Trim(b.OID, "0") == ""
}

type blameMeta struct {
	author   string
	unix     int64
	tz       string
	summary  string
	filename string
}

func (m *blameMeta) when() time.Time {
	t := time.Unix(m.unix, 0)
	if len(m.tz) != tzLength {
		return t
	}
	hours, errH := strconv.Atoi(m.tz[1:3])
	minutes, errM := strconv.Atoi(m.tz[3:5])
	if errH != nil || errM != nil {
		return t
	}
	offset := hours*secondsPerHour + minutes*secondsPerMinute
	if m.tz[0] == '-' {
		offset = -offset
	}
	return t.In(time.FixedZone("", offset))
}

func (r *Repo) Blame(ctx context.Context, path, rev string, contents []byte) ([]BlameLine, error) {
	if err := checkPath(path); err != nil {
		return nil, err
	}
	args := []string{"blame", "--porcelain"}
	c := readOnly
	if contents != nil {
		args = append(args, "--contents", "-")
		c.stdin = bytes.NewReader(contents)
	}
	if rev != "" {
		if err := checkRef(rev); err != nil {
			return nil, err
		}
		args = append(args, rev)
	}
	out, err := r.run(ctx, c, append(args, "--", path)...)
	if err != nil {
		var ge *Error
		if errors.As(err, &ge) && strings.Contains(ge.Stderr, blameNoSuchPath) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return parseBlame(out)
}

func parseBlame(data []byte) ([]BlameLine, error) {
	var lines []BlameLine
	metas := make(map[string]*blameMeta)
	var cur *BlameLine
	for _, line := range strings.Split(string(data), "\n") {
		if cur == nil {
			if line == "" {
				continue
			}
			header, err := parseBlameHeader(line)
			if err != nil {
				return nil, err
			}
			cur = header
			if metas[cur.OID] == nil {
				metas[cur.OID] = &blameMeta{}
			}
			continue
		}
		if text, ok := strings.CutPrefix(line, "\t"); ok {
			m := metas[cur.OID]
			cur.Author, cur.Time, cur.Summary, cur.Filename = m.author, m.when(), m.summary, m.filename
			cur.Text = text
			lines = append(lines, *cur)
			cur = nil
			continue
		}
		key, value, _ := strings.Cut(line, " ")
		m := metas[cur.OID]
		switch key {
		case "author":
			m.author = value
		case "author-time":
			n, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("git blame: malformed author-time %q", value)
			}
			m.unix = n
		case "author-tz":
			m.tz = value
		case "summary":
			m.summary = value
		case "filename":
			m.filename = value
		}
	}
	if cur != nil {
		return nil, errors.New("git blame: truncated output")
	}
	return lines, nil
}

func parseBlameHeader(line string) (*BlameLine, error) {
	f := strings.Fields(line)
	if len(f) < blameHeaderMinFields || len(f) > blameHeaderMaxFields || len(f[0]) < minOIDLength {
		return nil, fmt.Errorf("git blame: malformed header %q", line)
	}
	orig, errO := strconv.Atoi(f[1])
	final, errF := strconv.Atoi(f[2])
	if errO != nil || errF != nil {
		return nil, fmt.Errorf("git blame: malformed header %q", line)
	}
	return &BlameLine{OID: f[0], OrigLine: orig, Line: final}, nil
}
