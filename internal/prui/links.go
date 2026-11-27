package prui

import (
	"fmt"
	"strings"

	"github.com/ZerGo0/editor/internal/github"
)

const (
	devHost         = "github.dev"
	suggestionFence = "```suggestion"
	closingFence    = "```"
	crlf            = "\r\n"
	lf              = "\n"
)

func blobURL(repo github.Remote, ref, path string, start, end int) string {
	host := repo.Host
	if host == "" {
		host = github.DotComHost
	}
	return lineAnchor(fmt.Sprintf("https://%s/%s/%s/blob/%s/%s", host, repo.Owner, repo.Name, escapePath(ref), escapePath(path)), start, end)
}

func devURL(repo github.Remote, ref, path string, start, end int) (string, bool) {
	if repo.Host != "" && repo.Host != github.DotComHost {
		return "", false
	}
	return lineAnchor(fmt.Sprintf("https://%s/%s/%s/blob/%s/%s", devHost, repo.Owner, repo.Name, escapePath(ref), escapePath(path)), start, end), true
}

func lineAnchor(url string, start, end int) string {
	switch {
	case start > 0 && end > start:
		return fmt.Sprintf("%s#L%d-L%d", url, start, end)
	case start > 0:
		return fmt.Sprintf("%s#L%d", url, start)
	}
	return url
}

func markdownPermalink(path string, start, end int, url string) string {
	label := path
	switch {
	case start > 0 && end > start:
		label = fmt.Sprintf("%s#L%d-L%d", path, start, end)
	case start > 0:
		label = fmt.Sprintf("%s#L%d", path, start)
	}
	return fmt.Sprintf("[%s](%s)", label, url)
}

func suggestionOf(body string) (string, bool) {
	_, rest, found := strings.Cut(body, suggestionFence)
	if !found {
		return "", false
	}
	_, rest, found = strings.Cut(rest, lf)
	if !found {
		return "", false
	}
	code, _, closed := strings.Cut(rest, closingFence)
	if !closed {
		return "", false
	}
	return strings.TrimSuffix(strings.TrimSuffix(code, lf), "\r"), true
}

func applySuggestionText(text string, start, end int, code string) (string, error) {
	eol := lf
	if strings.Contains(text, crlf) {
		eol = crlf
	}
	lines := strings.Split(text, eol)
	count := len(lines)
	if count > 0 && lines[count-1] == "" {
		count--
	}
	if start < 1 || end < start || end > count {
		return "", fmt.Errorf("lines %d to %d are outside the file (%d lines)", start, end, count)
	}
	replacement := strings.Split(strings.ReplaceAll(code, crlf, lf), lf)
	out := append(append(append([]string{}, lines[:start-1]...), replacement...), lines[end:]...)
	return strings.Join(out, eol), nil
}
