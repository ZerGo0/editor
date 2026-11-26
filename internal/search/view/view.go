package searchview

import (
	"context"
	"strings"
	"time"

	"github.com/ZerGo0/editor/internal/host"
	"github.com/ZerGo0/editor/internal/search/everywhere"
	"github.com/egoist/mygo/ui"
)

const (
	quickDelay    = 40 * time.Millisecond
	textDelay     = 300 * time.Millisecond
	flushInterval = 16 * time.Millisecond
	minAllText    = 2
	allTextLimit  = 100
	ResultLimit   = 20000
)

type section struct {
	items   []everywhere.Result
	total   int
	loading bool
}

type View struct {
	env     Env
	h       host.Host
	st      State
	confirm func(message, detail, action string, done func(ok bool))

	rows        []row
	rowsDirty   bool
	collapsed   map[string]bool
	list        ui.ListState
	selected    int
	selectedKey string
	scrollTo    bool

	sections   [scopeCount]section
	text       textResults
	editorOnly bool
	sess       *everywhere.Session

	quickGen    int
	quickCancel context.CancelFunc
	textRun     textRunner

	scope    Scope
	quickArm bool
	textArm  bool
	quickDue time.Time
	textDue  time.Time
	quickGap time.Duration
	textGap  time.Duration

	field                 ui.Handle
	replaceField          ui.Handle
	includeField          ui.Handle
	pendingNewline        bool
	pendingReplaceNewline bool
	openPending           bool
	userMoved             bool
	upDown                bool
	refreshing            int
	historyPos            int
	historyDraft          string
	lastPreview           string
	cursor                string
	replacing             bool
}

func New(env Env) *View {
	v := &View{env: env, h: env.Host, collapsed: map[string]bool{}, selected: -1, historyPos: -1, rowsDirty: true}
	v.confirm = v.defaultConfirm
	v.list.Key = func(i int) any { return v.rowKey(i) }
	v.list.Header = func(i int) bool { return i >= 0 && i < len(v.rows) && v.rows[i].kind == rowHeader }
	v.list.Label = func(i int) string {
		if i < 0 || i >= len(v.rows) {
			return ""
		}
		return v.rows[i].label
	}
	v.list.Selected = &v.selected
	return v
}

func (v *View) rowKey(i int) any {
	if i < 0 || i >= len(v.rows) {
		return i
	}
	return v.rows[i].key
}

func (v *View) post(fn func()) {
	v.h.Update(fn)
}

func (v *View) Snapshot() State {
	st := v.st
	st.Scope = v.scope.String()
	st.History = append([]string(nil), st.History...)
	return st
}

func (v *View) Restore(st State) {
	v.cancelAll()
	v.st = st
	v.scope = parseScope(st.Scope)
	v.text = textResults{}
	v.sections = [scopeCount]section{}
	v.selected, v.selectedKey = -1, ""
	v.rowsDirty = true
	if v.scope != ScopeAll || strings.TrimSpace(st.Query) != "" {
		v.searchSoon(0)
	}
}

func (v *View) Count() int {
	return v.text.total
}

func (v *View) Focus(scope Scope, seed string) {
	v.sess = nil
	v.userMoved = false
	v.setScope(scope)
	if seed != "" {
		v.st.Query = seed
	}
	v.field.SelectAll()
	v.searchSoon(0)
}

func (v *View) FocusEditorSymbols() {
	v.Focus(ScopeSymbols, "")
	v.editorOnly = true
	v.st.Query = ""
	v.searchSoon(0)
}

func (v *View) setScope(s Scope) {
	if s < 0 || s >= scopeCount {
		s = ScopeAll
	}
	v.editorOnly = false
	v.scope = s
	v.rowsDirty = true
}

func (v *View) cancelAll() {
	if v.quickCancel != nil {
		v.quickCancel()
	}
	v.cancelText()
	v.quickDue, v.textDue = time.Time{}, time.Time{}
	v.quickArm, v.textArm = false, false
}

func (v *View) session() *everywhere.Session {
	if v.sess == nil && v.env.Providers != nil {
		v.sess = v.env.Providers.Session()
		v.sess.MarkOpen(v.env.openFiles())
	}
	return v.sess
}

func (v *View) searchSoon(delay time.Duration) {
	v.quickArm, v.quickGap = true, min(delay, quickDelay)
	v.textArm, v.textGap = v.needsText(), delay
	if !v.textArm && v.text.matcher != nil {
		v.cancelText()
		v.text = textResults{}
		v.rowsDirty = true
	}
}

func (v *View) needsText() bool {
	switch v.scope {
	case ScopeText:
		return true
	case ScopeAll:
		return len([]rune(strings.TrimSpace(v.st.Query))) >= minAllText
	}
	return false
}

func (v *View) tick(c *ui.Context) {
	now := c.Now()
	if v.quickArm {
		v.quickArm = false
		v.quickDue = now.Add(v.quickGap)
	}
	if v.textArm {
		v.textArm = false
		v.textDue = now.Add(v.textGap)
	}
	if !v.quickDue.IsZero() {
		if now.Before(v.quickDue) {
			c.After(v.quickDue.Sub(now))
		} else {
			v.quickDue = time.Time{}
			v.runQuick()
		}
	}
	if !v.textDue.IsZero() {
		if now.Before(v.textDue) {
			c.After(v.textDue.Sub(now))
		} else {
			v.textDue = time.Time{}
			v.runText()
		}
	}
}
