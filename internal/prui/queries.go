package prui

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/github"
)

const (
	queriesFile     = "github-pr-queries.json"
	queriesFileMode = 0o644
	queriesDirMode  = 0o755
	maxQueries      = 50
)

type savedQuery struct {
	Label string `json:"label"`
	Query string `json:"query"`
}

func queriesPath(dir string) string {
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, queriesFile)
}

func loadQueries(path string) ([]savedQuery, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("prui: read saved queries: %w", err)
	}
	var queries []savedQuery
	if err := json.Unmarshal(data, &queries); err != nil {
		return nil, fmt.Errorf("prui: decode saved queries: %w", err)
	}
	valid := queries[:0]
	for _, q := range queries {
		if strings.TrimSpace(q.Label) != "" && strings.TrimSpace(q.Query) != "" && len(valid) < maxQueries {
			valid = append(valid, q)
		}
	}
	return valid, nil
}

func saveQueries(path string, queries []savedQuery) error {
	if path == "" {
		return nil
	}
	data, err := json.MarshalIndent(queries, "", "  ")
	if err != nil {
		return fmt.Errorf("prui: encode saved queries: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), queriesDirMode); err != nil {
		return fmt.Errorf("prui: create queries folder: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, queriesFileMode); err != nil {
		return fmt.Errorf("prui: write saved queries: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("prui: replace saved queries: %w", err)
	}
	return nil
}

func expandQuery(query string, repo github.Remote) string {
	r := strings.NewReplacer("${owner}", repo.Owner, "${repository}", repo.Name, "${user}", selfQualifier)
	words := strings.Fields(r.Replace(query))
	own := "repo:" + repo.Owner + "/" + repo.Name
	kept := words[:0]
	for _, w := range words {
		if !strings.EqualFold(w, own) {
			kept = append(kept, w)
		}
	}
	return strings.Join(kept, " ")
}

func (p *PRs) rebuildCustom() {
	custom := make([]*section, 0, len(p.queries))
	for i, q := range p.queries {
		var prev *section
		for _, old := range p.side.custom {
			if old.title == q.Label && old.query == q.Query {
				prev = old
			}
		}
		if prev != nil {
			custom = append(custom, prev)
			continue
		}
		custom = append(custom, &section{id: fmt.Sprintf("query-%d", i), title: q.Label, query: q.Query, opts: github.ListOptions{Query: expandQuery(q.Query, p.repo)}})
	}
	p.side.custom = custom
}

func (p *PRs) persistQueries() {
	path := queriesPath(p.h.UserDataDir())
	queries := append([]savedQuery(nil), p.queries...)
	p.queriesSeq++
	seq := p.queriesSeq
	applog.Go(func() {
		p.queriesMu.Lock()
		defer p.queriesMu.Unlock()
		if seq < p.queriesDone {
			return
		}
		p.queriesDone = seq
		if err := saveQueries(path, queries); err != nil {
			p.post(func() { p.h.ShowError("Cannot save the queries", err) })
		}
	})
}

func (p *PRs) addQuery(label, query string) error {
	label, query = trimmed(label), trimmed(query)
	switch {
	case label == "" || query == "":
		return errors.New("enter a label and a query")
	case len(p.queries) >= maxQueries:
		return fmt.Errorf("at most %d queries can be saved", maxQueries)
	}
	for _, q := range p.queries {
		if strings.EqualFold(q.Label, label) {
			return fmt.Errorf("a query named %q exists", label)
		}
	}
	p.queries = append(p.queries, savedQuery{Label: label, Query: query})
	p.rebuildCustom()
	p.persistQueries()
	return nil
}

func (p *PRs) editQuery(sec *section, query string) error {
	query = trimmed(query)
	if query == "" {
		return errors.New("enter a query")
	}
	for i := range p.queries {
		if p.queries[i].Label == sec.title && p.queries[i].Query == sec.query {
			p.queries[i].Query = query
		}
	}
	p.rebuildCustom()
	p.persistQueries()
	return nil
}

func (p *PRs) removeQuery(sec *section) {
	for i := range p.queries {
		if p.queries[i].Label == sec.title && p.queries[i].Query == sec.query {
			p.queries = append(p.queries[:i], p.queries[i+1:]...)
			break
		}
	}
	p.rebuildCustom()
	p.persistQueries()
}
