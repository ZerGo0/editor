package workspace

import (
	"encoding/json"
	"maps"
	"math"
	"slices"

	"github.com/ZerGo0/editor/internal/design/badge"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/layout"
	"github.com/ZerGo0/editor/internal/settings"
	"github.com/ZerGo0/editor/internal/unicodehighlight"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	unicodeMargin   = bracketMargin
	osLocales       = "_os"
	appLocale       = "_vscode"
	invisibleSample = " "
)

var proseLanguages = map[string]bool{"Markdown": true, "Plain Text": true}

type unicodeState struct {
	views   map[*editor.State]unicodeView
	widths  map[unicodeWidthKey]float32
	allowed string
	locales string
	opts    unicodehighlight.Options
}

type unicodeWidthKey struct {
	text string
	font ui.Font
}

type unicodeConfig struct {
	control, invisible, ambiguous, nonBasic bool
	allowed, locales                        string
	font                                    ui.Font
	dark                                    bool
}

type unicodeView struct {
	version  uint64
	from, to int
	config   unicodeConfig
}

func (w *Workspace) decorateCode(c *ui.Context, tab *layout.Tab) {
	st := tab.Editor
	brackets, bracketsChanged := w.colorBrackets(c, tab)
	marks, marksChanged := w.markUnicode(c, st)
	if !bracketsChanged && !marksChanged {
		return
	}
	current := st.Replacements()
	if !bracketsChanged {
		brackets = keepReplacements(current, isBracketColor)
	}
	if !marksChanged {
		marks = keepReplacements(current, func(r editor.Replacement) bool { return !isBracketColor(r) })
	}
	st.SetReplacements(append(brackets, marks...))
}

func keepReplacements(rs []editor.Replacement, keep func(editor.Replacement) bool) []editor.Replacement {
	var out []editor.Replacement
	for _, r := range rs {
		if keep(r) {
			out = append(out, r)
		}
	}
	return out
}

func (w *Workspace) unicodeConfig(c *ui.Context, st *editor.State) unicodeConfig {
	s := w.settings
	cfg := unicodeConfig{
		control:   s.Bool(settings.ControlCharacters),
		invisible: s.Bool(settings.UnicodeInvisible),
		ambiguous: s.Bool(settings.UnicodeAmbiguous),
		nonBasic:  s.String(settings.UnicodeNonBasicASCII) == settings.UnicodeOn,
		allowed:   string(s.JSON(settings.UnicodeAllowedChars)),
		locales:   string(s.JSON(settings.UnicodeAllowedLocale)),
		font:      st.Font,
		dark:      token.Of(c).Dark,
	}
	if lang := st.Buffer.Language(); lang == nil || proseLanguages[lang.Name] {
		cfg.invisible, cfg.ambiguous = false, false
	}
	return cfg
}

func (w *Workspace) markUnicode(c *ui.Context, st *editor.State) ([]editor.Replacement, bool) {
	u := &w.unicode
	if u.views == nil {
		u.views, u.widths = map[*editor.State]unicodeView{}, map[unicodeWidthKey]float32{}
	}
	cfg := w.unicodeConfig(c, st)
	if !cfg.control && !cfg.invisible && !cfg.ambiguous && !cfg.nonBasic {
		if _, ok := u.views[st]; ok {
			delete(u.views, st)
			return nil, true
		}
		return nil, false
	}
	buf := st.Buffer
	version := buf.Version()
	first, last := st.VisibleLines()
	v, ok := u.views[st]
	if ok && v.version == version && v.config == cfg && first >= v.from && last <= v.to {
		return nil, false
	}
	v = unicodeView{version: version, config: cfg, from: max(0, first-unicodeMargin), to: last + unicodeMargin}
	u.views[st] = v
	o := u.options(cfg, func(tag string) []string { return systemLanguages(c, tag, w.win != nil) })
	snap := buf.Snapshot()
	var marks []unicodehighlight.Mark
	for line := v.from; line <= min(v.to, snap.LineCount()-1); line++ {
		marks = unicodehighlight.Scan(snap.Line(line), snap.LineStart(line), o, marks)
	}
	rs := make([]editor.Replacement, 0, len(marks))
	for _, m := range marks {
		rs = append(rs, u.replacement(c, m, cfg.font))
	}
	return rs, true
}

func (u *unicodeState) options(cfg unicodeConfig, system func(tag string) []string) *unicodehighlight.Options {
	if cfg.allowed != u.allowed || u.opts.Allowed == nil {
		u.allowed, u.opts.Allowed = cfg.allowed, allowedCharacters(cfg.allowed)
	}
	if cfg.locales != u.locales {
		u.locales, u.opts.Locales = cfg.locales, unicodehighlight.ForLanguages(allowedLanguages(cfg.locales, system))
	}
	u.opts.Control, u.opts.Invisible, u.opts.Ambiguous, u.opts.NonBasicASCII = cfg.control, cfg.invisible, cfg.ambiguous, cfg.nonBasic
	return &u.opts
}

func allowedCharacters(raw string) map[rune]bool {
	var set map[string]bool
	_ = json.Unmarshal([]byte(raw), &set)
	out := map[rune]bool{}
	for s, on := range set {
		for _, r := range s {
			out[r] = on
		}
	}
	return out
}

func systemLanguages(c *ui.Context, tag string, attached bool) []string {
	switch {
	case tag == appLocale:
		return []string{c.Locale()}
	case attached:
		return mygo.App.PreferredLanguages()
	}
	return nil
}

func allowedLanguages(raw string, system func(tag string) []string) []string {
	var set map[string]bool
	_ = json.Unmarshal([]byte(raw), &set)
	var out []string
	for _, tag := range slices.Sorted(maps.Keys(set)) {
		switch {
		case !set[tag]:
		case tag == osLocales || tag == appLocale:
			out = append(out, system(tag)...)
		default:
			out = append(out, tag)
		}
	}
	return out
}

func (u *unicodeState) replacement(c *ui.Context, m unicodehighlight.Mark, font ui.Font) editor.Replacement {
	text, sample, tone := string(m.Rune), string(m.Rune), token.ToneWarning
	switch m.Kind {
	case unicodehighlight.Control:
		text, tone = unicodehighlight.ControlText(m.Rune), token.ToneError
		sample = text
	case unicodehighlight.Invisible:
		text, sample = "", invisibleSample
	}
	key := unicodeWidthKey{sample, font}
	width, ok := u.widths[key]
	if !ok {
		width = float32(math.Ceil(float64(badge.CodeMarkWidth(c, sample, font))))
		u.widths[key] = width
	}
	label := unicodehighlight.Describe(m)
	return editor.Replacement{Start: m.Start, End: m.End, Width: width, Label: label,
		Content: func(c *ui.Context, e ui.Element, _ editor.Replacement) {
			badge.CodeMark(c, e, text, badge.CodeMarkOptions{Font: font, Tone: tone, Tooltip: label})
		}}
}
