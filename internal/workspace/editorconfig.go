package workspace

import (
	"log"
	"os"
	"path/filepath"
	"slices"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/document"
	"github.com/ZerGo0/editor/internal/editorconfig"
	"github.com/ZerGo0/editor/internal/fsutil"
	"github.com/ZerGo0/editor/internal/settings"
	"github.com/egoist/mygo/ui/editor"
)

type editorConfigState struct {
	byPath  map[string]editorconfig.Settings
	files   map[string]fileStamp
	seq     uint64
	applied map[string]uint64
	noticed map[string]bool
}

type editorConfigResult struct {
	path     string
	settings editorconfig.Settings
	err      error
}

func (w *Workspace) initEditorConfig() {
	w.ec = editorConfigState{
		byPath:  map[string]editorconfig.Settings{},
		files:   map[string]fileStamp{},
		applied: map[string]uint64{},
		noticed: map[string]bool{},
	}
}

func resolveEditorConfig(paths []string) ([]editorConfigResult, map[string]fileStamp) {
	results := make([]editorConfigResult, 0, len(paths))
	files := map[string]fileStamp{}
	for _, p := range paths {
		props, consulted, err := editorconfig.Resolve(p, os.ReadFile)
		results = append(results, editorConfigResult{path: p, settings: props.Settings(), err: err})
		for _, f := range consulted {
			if _, ok := files[f]; !ok {
				files[f], _ = statFile(f)
			}
		}
	}
	return results, files
}

func (w *Workspace) refreshEditorConfig(paths []string) {
	paths = slices.DeleteFunc(slices.Clone(paths), func(p string) bool { return !onDisk(p) })
	if len(paths) == 0 {
		return
	}
	w.ec.seq++
	seq := w.ec.seq
	if w.win == nil {
		results, files := resolveEditorConfig(paths)
		w.applyEditorConfigResults(seq, results, files)
		return
	}
	applog.Go(func() {
		results, files := resolveEditorConfig(paths)
		w.update(func() { w.applyEditorConfigResults(seq, results, files) })
	})
}

func (w *Workspace) applyEditorConfigResults(seq uint64, results []editorConfigResult, files map[string]fileStamp) {
	for f, s := range files {
		w.ec.files[f] = s
	}
	for _, r := range results {
		if w.ec.applied[r.path] > seq {
			continue
		}
		w.ec.applied[r.path] = seq
		if r.err != nil {
			log.Printf("editorconfig: %v", r.err)
		}
		if r.settings.IsZero() {
			delete(w.ec.byPath, r.path)
			continue
		}
		w.ec.byPath[r.path] = r.settings
		cs := r.settings.Charset
		if cs != "" && cs != editorconfig.CharsetUTF8 && !w.ec.noticed[r.path] {
			w.ec.noticed[r.path] = true
			w.setNotice("EditorConfig sets charset " + cs + " for " + filepath.Base(r.path) + ". Only UTF-8 is supported.")
		}
	}
}

func (w *Workspace) editorConfigChanged(dirs []string) {
	if len(w.ec.files) == 0 {
		return
	}
	var watched []string
	old := map[string]fileStamp{}
	for f, s := range w.ec.files {
		if slices.ContainsFunc(dirs, func(d string) bool { return fsutil.Within(d, f) }) {
			watched = append(watched, f)
			old[f] = s
		}
	}
	if len(watched) == 0 {
		return
	}
	applog.Go(func() {
		for _, f := range watched {
			if s, _ := statFile(f); !s.same(old[f]) {
				w.update(w.refreshAllEditorConfig)
				return
			}
		}
	})
}

func (w *Workspace) refreshAllEditorConfig() {
	clear(w.ec.files)
	paths := make([]string, 0, len(w.docs))
	for p := range w.docs {
		paths = append(paths, p)
	}
	w.refreshEditorConfig(paths)
}

func (w *Workspace) applyEditorConfig(st *editor.State, path string) {
	s, ok := w.ec.byPath[path]
	if !ok {
		return
	}
	switch s.IndentStyle {
	case editorconfig.IndentTabs:
		st.Indent = editor.IndentTabs
	case editorconfig.IndentSpaces:
		st.Indent = editor.IndentSpaces
	}
	size := s.IndentSize
	if st.Indent == editor.IndentTabs {
		size = s.TabWidth
	}
	if size > 0 {
		st.TabSize = size
	}
	if s.MaxLineLength > 0 {
		st.WrapGuides = []int{s.MaxLineLength}
	}
}

func editorConfigToggle(t editorconfig.Toggle, fallback bool) bool {
	switch t {
	case editorconfig.On:
		return true
	case editorconfig.Off:
		return false
	}
	return fallback
}

func (w *Workspace) saveRules(d *document.Document) (trim, finalNewline bool, lineEnding string) {
	s := w.ec.byPath[d.Path]
	trim = editorConfigToggle(s.TrimTrailingWhitespace, w.settings.Bool(settings.TrimTrailing))
	finalNewline = editorConfigToggle(s.InsertFinalNewline, w.settings.Bool(settings.FinalNewline))
	return trim, finalNewline, s.EndOfLine
}

func (w *Workspace) dropUnusedEditorConfig(open map[string]bool) {
	for p := range w.ec.applied {
		if !open[p] {
			delete(w.ec.applied, p)
			delete(w.ec.byPath, p)
			delete(w.ec.noticed, p)
		}
	}
}
