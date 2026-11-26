package gitui

import (
	"fmt"
	"path"
	"strings"
	"time"
)

const (
	shortHashLength = 8
	hoursPerDay     = 24
	clockFormat     = "15:04"
	sameYearFormat  = "Jan 2 15:04"
	otherYearFormat = "Jan 2, 2006"
	headRefMarker   = "HEAD -> "
	tagRefMarker    = "tag: "
	detachedRef     = "HEAD"
	ignoreFileName  = ".gitignore"
)

func shortHash(oid string) string {
	if len(oid) > shortHashLength {
		return oid[:shortHashLength]
	}
	return oid
}

func formatTime(t, now time.Time) string {
	if t.IsZero() {
		return ""
	}
	t = t.In(now.Location())
	y, m, d := now.Date()
	today := time.Date(y, m, d, 0, 0, 0, 0, now.Location())
	switch {
	case !t.Before(today):
		return "Today " + t.Format(clockFormat)
	case !t.Before(today.Add(-hoursPerDay * time.Hour)):
		return "Yesterday " + t.Format(clockFormat)
	case t.Year() == now.Year():
		return t.Format(sameYearFormat)
	}
	return t.Format(otherYearFormat)
}

func statusName(code byte) string {
	switch code {
	case 'A':
		return "Added"
	case 'D':
		return "Deleted"
	case 'R':
		return "Renamed"
	case 'C':
		return "Copied"
	case '?':
		return "Untracked"
	case 'U':
		return "Conflict"
	}
	return "Modified"
}

func splitPath(rel string) (name, dir string) {
	dir, name = path.Split(rel)
	return name, strings.TrimSuffix(dir, "/")
}

func syncLabel(ahead, behind int) string {
	var parts []string
	if ahead > 0 {
		parts = append(parts, fmt.Sprintf("↑%d", ahead))
	}
	if behind > 0 {
		parts = append(parts, fmt.Sprintf("↓%d", behind))
	}
	return strings.Join(parts, " ")
}

type refKind int

const (
	refLocal refKind = iota
	refHead
	refRemote
	refTag
)

type refBadge struct {
	name string
	kind refKind
}

func refBadges(refs []string, remotes map[string]bool) []refBadge {
	var out []refBadge
	for _, r := range refs {
		switch {
		case r == detachedRef:
			out = append(out, refBadge{name: r, kind: refHead})
		case strings.HasPrefix(r, headRefMarker):
			out = append(out, refBadge{name: strings.TrimPrefix(r, headRefMarker), kind: refHead})
		case strings.HasPrefix(r, tagRefMarker):
			out = append(out, refBadge{name: strings.TrimPrefix(r, tagRefMarker), kind: refTag})
		case isRemoteRef(r, remotes):
			out = append(out, refBadge{name: r, kind: refRemote})
		default:
			out = append(out, refBadge{name: r, kind: refLocal})
		}
	}
	return out
}

func isRemoteRef(ref string, remotes map[string]bool) bool {
	remote, _, ok := strings.Cut(ref, "/")
	return ok && remotes[remote]
}

func appendIgnore(existing, rel string) string {
	entry := "/" + strings.TrimPrefix(rel, "/")
	for _, line := range strings.Split(existing, "\n") {
		if strings.TrimSpace(line) == entry {
			return existing
		}
	}
	if existing != "" && !strings.HasSuffix(existing, "\n") {
		existing += "\n"
	}
	return existing + entry + "\n"
}

func firstLineOf(s string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	return line
}
