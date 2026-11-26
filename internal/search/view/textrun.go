package searchview

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/ZerGo0/editor/internal/fsutil"
	"github.com/ZerGo0/editor/internal/search/textsearch"
	"github.com/egoist/mygo/ui/editor"
)

type textResults struct {
	files     []*fileNode
	total     int
	truncated bool
	searching bool
	fresh     bool
	err       string
	scanned   int
	matcher   *textsearch.Matcher
	query     string
}

type textRunner struct {
	mu        sync.Mutex
	cancel    context.CancelFunc
	gen       int
	pending   []textsearch.FileResult
	scheduled bool
}

func (v *View) cancelText() {
	if v.textRun.cancel != nil {
		v.textRun.cancel()
		v.textRun.cancel = nil
	}
	v.textRun.mu.Lock()
	v.textRun.gen++
	v.textRun.pending, v.textRun.scheduled = nil, false
	v.textRun.mu.Unlock()
	v.text.searching = false
}

func (v *View) CancelSearch() {
	v.cancelText()
	v.text.fresh = false
	v.rowsDirty = true
}

func (v *View) textQuery() textsearch.Query {
	excludes := textsearch.ParseGlobs(v.st.Exclude)
	if !v.st.NoExcludes {
		excludes = append(excludes, v.env.excludePatterns()...)
	}
	return textsearch.Query{
		Pattern:   v.st.Query,
		MatchCase: v.st.MatchCase,
		WholeWord: v.st.WholeWord,
		Regex:     v.st.Regex,
		SmartCase: v.env.smartCase(),
		Root:      v.h.RootDir(),
		Include:   textsearch.ParseGlobs(v.st.Include),
		Exclude:   excludes,
	}
}

func patternMessage(err error) string {
	var pe *textsearch.PatternError
	if errors.As(err, &pe) {
		return pe.Err.Error()
	}
	return err.Error()
}

func (v *View) overlay() map[string]editor.Snapshot {
	out := map[string]editor.Snapshot{}
	for _, p := range v.env.openFiles() {
		if buf := v.h.Buffer(p); buf != nil {
			out[p] = buf.Snapshot()
		}
	}
	return out
}

func overlayFunc(snaps map[string]editor.Snapshot) func(string) (string, bool) {
	return func(p string) (string, bool) {
		s, ok := snaps[p]
		if !ok {
			return "", false
		}
		return s.String(), true
	}
}

func (v *View) runText() {
	v.cancelText()
	v.cursor = ""
	if v.st.Query == "" || strings.TrimSpace(v.st.Query) == "" {
		v.text = textResults{}
		v.rowsDirty = true
		return
	}
	m, err := textsearch.Compile(v.textQuery())
	if err != nil {
		v.text = textResults{err: patternMessage(err), query: v.st.Query}
		v.rowsDirty = true
		return
	}
	limit := ResultLimit
	if v.scope == ScopeAll {
		limit = allTextLimit
	}
	ctx, cancel := context.WithCancel(context.Background())
	v.textRun.cancel = cancel
	v.textRun.mu.Lock()
	gen := v.textRun.gen
	v.textRun.mu.Unlock()
	v.text.matcher, v.text.query = m, v.st.Query
	v.text.searching, v.text.fresh, v.text.err = true, true, ""
	snaps := v.overlay()
	root := v.h.RootDir()
	source := v.fileSource(m, snaps)
	opts := textsearch.Options{Root: root, Overlay: overlayFunc(snaps), Limit: limit}
	v.rowsDirty = true
	go func() {
		opts.Files = source(ctx)
		stats, err := textsearch.Search(ctx, m, opts, func(fr textsearch.FileResult) { v.enqueue(gen, fr) })
		if err != nil && ctx.Err() != nil {
			return
		}
		v.post(func() { v.finishText(gen, stats) })
	}()
}

func (v *View) fileSource(m *textsearch.Matcher, snaps map[string]editor.Snapshot) func(ctx context.Context) []string {
	root := v.h.RootDir()
	switch {
	case v.st.OnlyOpen:
		open := v.env.openFiles()
		return func(context.Context) []string { return open }
	case !v.st.NoExcludes && v.env.useIgnoreFiles() && v.env.Files != nil:
		index := v.env.Files
		return func(ctx context.Context) []string {
			rels, ready := index.Paths()
			if ready {
				return absPaths(root, rels)
			}
			return walkAbs(ctx, root, m)
		}
	}
	return func(ctx context.Context) []string { return walkAbs(ctx, root, m) }
}

func walkAbs(ctx context.Context, root string, m *textsearch.Matcher) []string {
	rels, _ := textsearch.WalkFiles(ctx, root, m.SkipDir)
	return absPaths(root, rels)
}

func absPaths(root string, rels []string) []string {
	out := make([]string, len(rels))
	for i, r := range rels {
		out[i] = filepath.Join(root, filepath.FromSlash(r))
	}
	return out
}

func (v *View) enqueue(gen int, fr textsearch.FileResult) {
	t := &v.textRun
	t.mu.Lock()
	if gen != t.gen {
		t.mu.Unlock()
		return
	}
	t.pending = append(t.pending, fr)
	schedule := !t.scheduled
	t.scheduled = true
	t.mu.Unlock()
	if schedule {
		time.AfterFunc(flushInterval, func() { v.post(func() { v.flushText(gen) }) })
	}
}

func (v *View) takePending(gen int) ([]textsearch.FileResult, bool) {
	t := &v.textRun
	t.mu.Lock()
	defer t.mu.Unlock()
	if gen != t.gen {
		return nil, false
	}
	out := t.pending
	t.pending, t.scheduled = nil, false
	return out, true
}

func (v *View) flushText(gen int) {
	batch, ok := v.takePending(gen)
	if !ok || len(batch) == 0 {
		return
	}
	if v.text.fresh {
		v.text.files, v.text.total, v.text.fresh = nil, 0, false
	}
	for _, fr := range batch {
		v.text.files = append(v.text.files, newFileNode(fr.Path, v.relPath(fr.Path), fr.Matches))
		v.text.total += len(fr.Matches)
	}
	v.sortFiles()
	v.rowsDirty = true
}

func (v *View) sortFiles() {
	slices.SortFunc(v.text.files, func(a, b *fileNode) int { return strings.Compare(a.rel, b.rel) })
}

func (v *View) finishText(gen int, stats textsearch.Stats) {
	v.flushText(gen)
	v.textRun.mu.Lock()
	current := gen == v.textRun.gen
	v.textRun.mu.Unlock()
	if !current {
		return
	}
	if v.text.fresh {
		v.text.files, v.text.total, v.text.fresh = nil, 0, false
	}
	v.text.searching, v.text.truncated, v.text.scanned = false, stats.Truncated, stats.Files
	if v.scope == ScopeText && v.text.total > 0 {
		v.st.History = addHistory(v.st.History, v.st.Query)
	}
	v.rowsDirty = true
}

func (v *View) Changed(paths []string) {
	if v.text.matcher == nil || v.text.err != "" || strings.TrimSpace(v.st.Query) == "" || v.scope != ScopeText && v.scope != ScopeAll {
		return
	}
	if v.text.searching || v.scope == ScopeAll {
		v.searchSoon(textDelay)
		return
	}
	v.refreshFiles(paths)
}

func (v *View) refreshFiles(paths []string) {
	m := v.text.matcher
	snaps := v.overlay()
	root := v.h.RootDir()
	gen := v.currentGen()
	set := make(map[string]bool, len(paths))
	for _, p := range paths {
		set[p] = true
	}
	for _, f := range v.text.files {
		if set[filepath.Dir(f.path)] {
			set[f.path] = true
		}
	}
	v.refreshing++
	go func() {
		files := expandDirs(paths)
		for p := range set {
			if !slices.Contains(files, p) && !isDir(p) {
				files = append(files, p)
			}
		}
		files = slices.DeleteFunc(files, func(p string) bool { return !fsutil.Within(root, p) })
		opts := textsearch.Options{Root: root, Overlay: overlayFunc(snaps), Files: files, Limit: ResultLimit}
		var found []textsearch.FileResult
		_, _ = textsearch.Search(context.Background(), m, opts, func(fr textsearch.FileResult) { found = append(found, fr) })
		v.post(func() {
			v.refreshing--
			if gen != v.currentGen() || v.text.matcher != m {
				return
			}
			for _, fr := range found {
				set[fr.Path] = true
			}
			v.replaceFiles(set, found)
		})
	}()
}

func isDir(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

func expandDirs(paths []string) []string {
	var out []string
	for _, p := range paths {
		if !isDir(p) {
			out = append(out, p)
			continue
		}
		entries, err := os.ReadDir(p)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.Type().IsRegular() {
				out = append(out, filepath.Join(p, e.Name()))
			}
		}
	}
	return out
}

func (v *View) currentGen() int {
	v.textRun.mu.Lock()
	defer v.textRun.mu.Unlock()
	return v.textRun.gen
}

func (v *View) replaceFiles(set map[string]bool, found []textsearch.FileResult) {
	v.text.files = slices.DeleteFunc(v.text.files, func(f *fileNode) bool { return set[f.path] })
	for _, fr := range found {
		v.text.files = append(v.text.files, newFileNode(fr.Path, v.relPath(fr.Path), fr.Matches))
	}
	v.sortFiles()
	v.recount()
	v.rowsDirty = true
}

func (v *View) recount() {
	v.text.total = 0
	for _, f := range v.text.files {
		v.text.total += len(f.matches)
	}
}
