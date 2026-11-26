package redline

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
)

const (
	allowSeparator = " -- "
	allowAnyView   = "*"
	allowFilePerm  = 0o644
)

type Allow struct {
	View   string
	Key    string
	Reason string
	used   bool
}

func (a *Allow) String() string {
	return a.View + " " + a.Key + allowSeparator + a.Reason
}

func (a *Allow) matches(view string) bool {
	return a.View == allowAnyView || strings.HasPrefix(view, a.View)
}

type Allowlist struct {
	Path    string
	Entries []*Allow
}

func LoadAllowlist(path string) (*Allowlist, error) {
	l := &Allowlist{Path: path}
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return l, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open redline allowlist: %w", err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		entry, reason, ok := strings.Cut(line, allowSeparator)
		view, key, okView := strings.Cut(entry, " ")
		if !ok || !okView || strings.TrimSpace(reason) == "" || strings.TrimSpace(key) == "" {
			return nil, fmt.Errorf("%s:%d: want \"<view|*> <finding key>%s<reason>\", got %q", path, n, allowSeparator, line)
		}
		l.Entries = append(l.Entries, &Allow{View: view, Key: strings.TrimSpace(key), Reason: strings.TrimSpace(reason)})
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read redline allowlist: %w", err)
	}
	return l, nil
}

func (l *Allowlist) Check(res Result) []int {
	var keys []string
	for _, a := range l.Entries {
		if !a.matches(res.Name) {
			continue
		}
		keys = append(keys, a.Key)
		for _, f := range res.Findings {
			if strings.HasPrefix(res.Key(f), a.Key) {
				a.used = true
			}
		}
	}
	return res.Unallowed(keys)
}

func (l *Allowlist) Stale() []string {
	var out []string
	for _, a := range l.Entries {
		if !a.used {
			out = append(out, a.String())
		}
	}
	return out
}

func (l *Allowlist) Prune() error {
	var kept []string
	for _, a := range l.Entries {
		if a.used {
			kept = append(kept, a.String())
		}
	}
	text := ""
	if len(kept) > 0 {
		text = strings.Join(kept, "\n") + "\n"
	}
	if err := os.WriteFile(l.Path, []byte(text), allowFilePerm); err != nil {
		return fmt.Errorf("write redline allowlist: %w", err)
	}
	return nil
}
