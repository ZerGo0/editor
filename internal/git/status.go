package git

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

const (
	statusUnchanged = '.'
	statusUntracked = '?'
	statusIgnored   = '!'

	detachedHead     = "(detached)"
	initialOID       = "(initial)"
	headerPrefix     = "# branch."
	statusCodeLength = 2

	changedFields   = 9
	renamedFields   = 10
	unmergedFields  = 11
	xyFieldIndex    = 1
	pathFieldIndex  = 2
	aheadPrefix     = "+"
	behindPrefix    = "-"
	aheadBehindArgs = 2
)

type FileStatus struct {
	Path     string
	OrigPath string
	Index    byte
	WorkTree byte
	Conflict bool
}

type Status struct {
	Branch   string
	Upstream string
	Ahead    int
	Behind   int
	Detached bool
	OID      string
	Files    []FileStatus
}

func (r *Repo) Status(ctx context.Context) (*Status, error) {
	out, err := r.read(ctx, "status", "--porcelain=v2", "-z", "--branch", "--untracked-files=all")
	if err != nil {
		return nil, err
	}
	return parseStatus(out)
}

func isTrackedCode(c byte) bool {
	return c != statusUnchanged && c != statusUntracked && c != statusIgnored
}

func (s *Status) Staged() []FileStatus {
	return s.filter(func(f FileStatus) bool { return !f.Conflict && isTrackedCode(f.Index) })
}

func (s *Status) Unstaged() []FileStatus {
	return s.filter(func(f FileStatus) bool { return !f.Conflict && isTrackedCode(f.WorkTree) })
}

func (s *Status) Untracked() []FileStatus {
	return s.filter(func(f FileStatus) bool { return f.Index == statusUntracked })
}

func (s *Status) Conflicted() []FileStatus {
	return s.filter(func(f FileStatus) bool { return f.Conflict })
}

func (s *Status) filter(keep func(FileStatus) bool) []FileStatus {
	var out []FileStatus
	for _, f := range s.Files {
		if keep(f) {
			out = append(out, f)
		}
	}
	return out
}

func parseStatus(data []byte) (*Status, error) {
	st := &Status{}
	records := splitNUL(data)
	for i := 0; i < len(records); i++ {
		rec := records[i]
		if rec == "" {
			continue
		}
		switch rec[0] {
		case '#':
			if err := parseStatusHeader(st, rec); err != nil {
				return nil, err
			}
		case '1':
			f, err := splitEntry(rec, changedFields)
			if err != nil {
				return nil, err
			}
			st.Files = append(st.Files, FileStatus{Path: f[changedFields-1], Index: f[xyFieldIndex][0], WorkTree: f[xyFieldIndex][1]})
		case '2':
			f, err := splitEntry(rec, renamedFields)
			if err != nil {
				return nil, err
			}
			if i+1 >= len(records) {
				return nil, fmt.Errorf("git status: rename entry %q has no original path", rec)
			}
			i++
			st.Files = append(st.Files, FileStatus{Path: f[renamedFields-1], OrigPath: records[i], Index: f[xyFieldIndex][0], WorkTree: f[xyFieldIndex][1]})
		case 'u':
			f, err := splitEntry(rec, unmergedFields)
			if err != nil {
				return nil, err
			}
			st.Files = append(st.Files, FileStatus{Path: f[unmergedFields-1], Index: f[xyFieldIndex][0], WorkTree: f[xyFieldIndex][1], Conflict: true})
		case '?', '!':
			if len(rec) < pathFieldIndex+1 || rec[1] != ' ' {
				return nil, fmt.Errorf("git status: malformed entry %q", rec)
			}
			st.Files = append(st.Files, FileStatus{Path: rec[pathFieldIndex:], Index: rec[0], WorkTree: rec[0]})
		default:
			return nil, fmt.Errorf("git status: unknown entry %q", rec)
		}
	}
	return st, nil
}

func splitEntry(rec string, fields int) ([]string, error) {
	f := strings.SplitN(rec, " ", fields)
	if len(f) != fields || len(f[xyFieldIndex]) != statusCodeLength || f[fields-1] == "" {
		return nil, fmt.Errorf("git status: malformed entry %q", rec)
	}
	return f, nil
}

func parseStatusHeader(st *Status, rec string) error {
	rest, ok := strings.CutPrefix(rec, headerPrefix)
	if !ok {
		return nil
	}
	key, value, _ := strings.Cut(rest, " ")
	switch key {
	case "oid":
		if value != initialOID {
			st.OID = value
		}
	case "head":
		if value == detachedHead {
			st.Detached = true
		} else {
			st.Branch = value
		}
	case "upstream":
		st.Upstream = value
	case "ab":
		parts := strings.Fields(value)
		if len(parts) != aheadBehindArgs {
			return fmt.Errorf("git status: malformed header %q", rec)
		}
		ahead, errA := strconv.Atoi(strings.TrimPrefix(parts[0], aheadPrefix))
		behind, errB := strconv.Atoi(strings.TrimPrefix(parts[1], behindPrefix))
		if errA != nil || errB != nil {
			return fmt.Errorf("git status: malformed header %q", rec)
		}
		st.Ahead, st.Behind = ahead, behind
	}
	return nil
}
