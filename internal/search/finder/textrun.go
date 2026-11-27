package finder

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/fsutil"
	"github.com/ZerGo0/editor/internal/search/textsearch"
	"github.com/egoist/mygo/ui/editor"
)

const (
	flushInterval = 16 * time.Millisecond
	resultLimit   = 20000
)

type textRequest struct {
	Pattern    string
	MatchCase  bool
	WholeWord  bool
	Regex      bool
	Include    []string
	Exclude    []string
	NonProject bool
	Directory  string
	Limit      int
}

type textRun struct {
	env       Env
	mu        sync.Mutex
	cancel    context.CancelFunc
	gen       int
	pending   []textsearch.FileResult
	scheduled bool

	files     []*FileNode
	total     int
	truncated bool
	searching bool
	fresh     bool
	renewed   bool
	err       string
	matcher   *textsearch.Matcher
	pattern   string
	version   int
}

func (r *textRun) stop() {
	if r.cancel != nil {
		r.cancel()
		r.cancel = nil
	}
	r.mu.Lock()
	r.gen++
	r.pending, r.scheduled = nil, false
	r.mu.Unlock()
	r.searching = false
}

func (r *textRun) clear() {
	r.files, r.total, r.truncated, r.err, r.matcher, r.pattern = nil, 0, false, "", nil, ""
	r.renewed = true
	r.version++
}

func (r *textRun) reset() {
	r.stop()
	if r.files != nil || r.err != "" || r.matcher != nil {
		r.clear()
	}
}

func (r *textRun) takeRenewed() bool {
	renewed := r.renewed
	r.renewed = false
	return renewed
}

func (r *textRun) currentGen() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.gen
}

func (r *textRun) root() string {
	return r.env.Host.RootDir()
}

func patternMessage(err error) string {
	var pe *textsearch.PatternError
	if errors.As(err, &pe) {
		return pe.Err.Error()
	}
	return err.Error()
}

func (r *textRun) compile(req textRequest) (*textsearch.Matcher, error) {
	excludes := slices.Clone(req.Exclude)
	if !req.NonProject {
		excludes = append(excludes, r.env.excludePatterns()...)
	}
	return textsearch.Compile(textsearch.Query{
		Pattern:   req.Pattern,
		MatchCase: req.MatchCase,
		WholeWord: req.WholeWord,
		Regex:     req.Regex,
		SmartCase: r.env.smartCase(),
		Root:      r.root(),
		Include:   req.Include,
		Exclude:   excludes,
	})
}

func (r *textRun) overlay() map[string]editor.Snapshot {
	out := map[string]editor.Snapshot{}
	for _, p := range r.env.openFiles() {
		if buf := r.env.Host.Buffer(p); buf != nil {
			out[p] = buf.Snapshot()
		}
	}
	return out
}

func (r *textRun) start(req textRequest) {
	r.stop()
	if strings.TrimSpace(req.Pattern) == "" {
		r.clear()
		return
	}
	m, err := r.compile(req)
	if err != nil {
		r.matcher = nil
		r.err, r.pattern = patternMessage(err), req.Pattern
		return
	}
	limit := req.Limit
	if limit <= 0 {
		limit = resultLimit
	}
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	gen := r.currentGen()
	r.matcher, r.pattern = m, req.Pattern
	r.searching, r.fresh, r.err = true, true, ""
	opts := textsearch.Options{Root: r.root(), Overlay: textsearch.SnapshotOverlay(r.overlay()), Limit: limit}
	source := r.fileSource(m, req)
	applog.Go(func() {
		opts.Files = source(ctx)
		stats, err := textsearch.Search(ctx, m, opts, func(fr textsearch.FileResult) { r.enqueue(gen, fr) })
		if err != nil && ctx.Err() != nil {
			return
		}
		r.env.Host.Update(func() { r.finish(gen, stats) })
	})
}

func (r *textRun) fileSource(m *textsearch.Matcher, req textRequest) func(ctx context.Context) []string {
	root := r.root()
	dir := root
	if req.Directory != "" {
		dir = req.Directory
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(root, dir)
		}
	}
	inDir := func(paths []string) []string {
		return slices.DeleteFunc(paths, func(p string) bool { return !fsutil.Within(dir, p) })
	}
	if !req.NonProject && r.env.useIgnoreFiles() && r.env.Files != nil {
		index := r.env.Files
		return func(ctx context.Context) []string {
			if rels, ready := index.Paths(); ready {
				return inDir(absPaths(root, rels))
			}
			return inDir(walkAbs(ctx, root, m))
		}
	}
	return func(ctx context.Context) []string { return inDir(walkAbs(ctx, root, m)) }
}

func walkAbs(ctx context.Context, root string, m *textsearch.Matcher) []string {
	rels, _ := textsearch.WalkFiles(ctx, root, m.SkipDir)
	return absPaths(root, rels)
}

func absPaths(root string, rels []string) []string {
	out := make([]string, len(rels))
	for i, rel := range rels {
		out[i] = filepath.Join(root, filepath.FromSlash(rel))
	}
	return out
}

func (r *textRun) enqueue(gen int, fr textsearch.FileResult) {
	r.mu.Lock()
	if gen != r.gen {
		r.mu.Unlock()
		return
	}
	r.pending = append(r.pending, fr)
	schedule := !r.scheduled
	r.scheduled = true
	r.mu.Unlock()
	if schedule {
		time.AfterFunc(flushInterval, func() { r.env.Host.Update(func() { r.flush(gen) }) })
	}
}

func (r *textRun) takePending(gen int) ([]textsearch.FileResult, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if gen != r.gen {
		return nil, false
	}
	out := r.pending
	r.pending, r.scheduled = nil, false
	return out, true
}

func (r *textRun) flush(gen int) {
	batch, ok := r.takePending(gen)
	if !ok || len(batch) == 0 {
		return
	}
	r.replaceIfFresh()
	slices.SortFunc(batch, func(a, b textsearch.FileResult) int { return strings.Compare(a.Path, b.Path) })
	for _, fr := range batch {
		r.files = append(r.files, NewFileNode(fr.Path, relTo(r.root(), fr.Path), fr.Matches))
		r.total += len(fr.Matches)
	}
	r.version++
}

func (r *textRun) replaceIfFresh() {
	if r.fresh {
		r.files, r.total, r.fresh, r.renewed = nil, 0, false, true
	}
}

func (r *textRun) finish(gen int, stats textsearch.Stats) {
	r.flush(gen)
	if gen != r.currentGen() {
		return
	}
	r.replaceIfFresh()
	r.searching, r.truncated = false, stats.Truncated
	r.version++
}
