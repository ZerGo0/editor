package review

import (
	"net/url"
	"strconv"
	"strings"
)

const (
	pullSegment   = "pull"
	minPRSegments = 4
)

func ParsePRURL(s string) (PRRef, bool) {
	u, err := url.Parse(s)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return PRRef{}, false
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < minPRSegments || parts[2] != pullSegment || parts[0] == "" || parts[1] == "" {
		return PRRef{}, false
	}
	n, err := strconv.Atoi(parts[3])
	if err != nil || n <= 0 {
		return PRRef{}, false
	}
	return PRRef{Host: u.Host, Owner: parts[0], Repo: parts[1], Number: n, URL: s}, true
}

func (p PRRef) Display() string { return p.Owner + "/" + p.Repo }
