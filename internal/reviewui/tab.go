package reviewui

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/ZerGo0/editor/internal/design"
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/dialog"
	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/ZerGo0/editor/internal/review"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	DecideID           = "review.decide"
	ApproveWithNotesID = "review.approveWithNotes"
	CloseID            = "review.close"
	AddGlobalCommentID = "review.addGlobalComment"

	titlePrefix        = "Review: "
	planTitle          = "Plan"
	toolbarLabel       = "Review Toolbar"
	diffTypeLabel      = "Diff Type"
	approveLabel       = "Approve"
	sendLabel          = "Send Feedback"
	approveNotesLabel  = "Approve with Notes"
	closeLabel         = "Close"
	decisionsMenuLabel = "More Decisions"
	loadingLabel       = "Loading the changes"
	cannotLoadTitle    = "Cannot load the changes"
	emptyTitle         = "No changes to review"
	emptyMessage       = "You can still approve, send feedback or close the review."
	decideErrorTitle   = "Cannot send the decision"
	diffPanePercent    = 72
	annotationNoun     = "annotation"
)

var errNoSource = errors.New("This window cannot load the changes to review.")

type confirmKind int

const (
	confirmNone confirmKind = iota
	confirmClose
	confirmDiscard
	confirmSwitch
)

type Tab struct {
	req     review.Request
	deps    Deps
	src     Source
	changes Changes
	pr      PullRequest
	mode    review.DiffMode
	pick    review.DiffMode
	notes   []note
	nextID  int
	inline  draft
	panel   draft
	states  map[string]*editor.DiffState
	heights map[any]float32
	split   float32
	confirm confirmKind
	dialog  bool
	done    bool
	result  review.Decision
	gone    bool
	err     error
}

func New(req review.Request, deps Deps) *Tab {
	t := &Tab{req: req, deps: deps, mode: req.Diff, pick: req.Diff, split: diffPanePercent,
		states: map[string]*editor.DiffState{}, heights: map[any]float32{}}
	t.open()
	return t
}

func (t *Tab) open() {
	switch {
	case t.req.Kind == review.KindReview && t.req.PR != nil:
		if t.deps.PR != nil {
			t.pr = t.deps.PR(*t.req.PR, t.decorate)
			t.src = t.pr
		}
	case t.req.Kind == review.KindReview:
		if t.deps.Changes != nil {
			base := ""
			if t.mode == review.DiffBranch {
				base = t.req.Base
			}
			t.changes = t.deps.Changes(t.mode, base, t.decorate)
			t.src = t.changes
		}
	default:
		t.src = newDocument(t.req, t.deps, t.decorate)
	}
}

func (t *Tab) Title() string {
	switch {
	case t.req.PR != nil:
		return titlePrefix + t.req.PR.Display() + "#" + strconv.Itoa(t.req.PR.Number)
	case t.req.Kind == review.KindReview:
		return titlePrefix + filepath.Base(t.req.Folder)
	case t.req.Kind == review.KindPlan:
		return titlePrefix + planTitle
	}
	return titlePrefix + filepath.Base(t.req.File)
}

func (t *Tab) Close() {
	if t.src != nil {
		t.src.Close()
	}
}

func (t *Tab) Done() bool { return t.done }

func (t *Tab) Composing() bool { return t.inline.open || t.panel.open }

func (t *Tab) Decide() {
	switch {
	case t.done:
	case len(t.notes) > 0:
		t.finish(review.Annotated, t.annotations())
	case t.req.CanApprove():
		t.finish(review.Approved, nil)
	}
}

func (t *Tab) ApproveWithNotes() {
	if !t.done && t.req.Notes && t.req.CanApprove() && len(t.notes) > 0 {
		t.finish(review.Approved, t.annotations())
	}
}

func (t *Tab) Dismiss() {
	if len(t.notes) > 0 {
		t.ask(confirmClose)
		return
	}
	t.finish(review.Dismissed, nil)
}

func (t *Tab) Abandon() {
	if !t.done {
		t.finish(review.Dismissed, nil)
	}
}

func (t *Tab) AddGlobalComment() {
	if !t.done {
		t.panel = draft{open: true, ann: review.Annotation{Scope: review.ScopeGeneral}}
	}
}

func (t *Tab) finish(d review.Decision, anns []review.Annotation) {
	if t.done {
		return
	}
	res := review.Result{Decision: d, Annotations: anns}
	if t.changes != nil {
		res.Diff, res.Base = t.mode, t.changes.Base()
	}
	if t.pr != nil {
		pr := t.pr.PR()
		res.PR = &pr
	}
	err := t.deps.Decide(res)
	switch {
	case errors.Is(err, review.ErrAgentGone):
		t.gone = true
	case err != nil:
		t.err = err
		return
	}
	t.done, t.result, t.err = true, d, nil
	t.inline, t.panel = draft{}, draft{}
}

func (t *Tab) ask(k confirmKind) {
	t.confirm, t.dialog = k, true
}

func (t *Tab) switchMode() {
	if t.changes != nil {
		t.changes.Close()
	}
	t.mode, t.notes, t.inline, t.panel = t.pick, nil, draft{}, draft{}
	clear(t.states)
	clear(t.heights)
	t.open()
}

func (t *Tab) View(c *ui.Context) {
	ui.Column(c).Fill().Children(func() {
		t.toolbar(c)
		if t.done {
			t.final(c)
			return
		}
		ui.SplitPercent(c, &t.split, func() {
			ui.Column(c).Fill().Children(func() { t.content(c) })
		}, func() {
			ui.Column(c).Fill().Children(func() { t.sidebar(c) })
		}).Fill()
		t.confirmDialog(c)
	})
	if t.pick != t.mode && !t.dialog {
		if len(t.notes) == 0 {
			t.switchMode()
		} else {
			t.ask(confirmSwitch)
		}
	}
}

func (t *Tab) content(c *ui.Context) {
	if t.src == nil {
		feedback.Error(c, cannotLoadTitle, errNoSource, nil)
		return
	}
	loaded, _, err := t.src.Status()
	switch {
	case err != nil:
		feedback.Error(c, cannotLoadTitle, err, t.src.Reload)
	case !loaded:
		feedback.LoadingPane(c, loadingLabel)
	case len(t.src.View().Entries()) == 0:
		feedback.Empty(c, feedback.EmptyOptions{Icon: icons.Check, Title: emptyTitle, Message: emptyMessage})
	default:
		t.src.View().Render(c, t.src.Options())
	}
}

func (t *Tab) toolbar(c *ui.Context) {
	p := token.Of(c)
	ui.Row(c).FillWidth().MinHeight(token.ToolbarHeight.Px()).Shrink(0).Padding(token.GapInline.Px(), token.PadRow.Px()).Gap(token.GapControl.Px()).
		AlignItems(ui.Center).BorderWidth(0, 0, token.BorderWidth.Px(), 0).BorderColor(p.Border.Default).
		Role(ui.RoleToolbar).Label(toolbarLabel).FocusGroup(ui.Horizontal).Children(func() {
		ui.Icon(c, icons.Comment).Size(token.IconSize.Px(), token.IconSize.Px()).TextColor(p.Icon).Shrink(0).AccessHidden(true)
		design.Text(c, t.subject(), token.TextStrong).SingleLine().Shrink(1).MinWidth(0)
		if t.changes != nil && !t.done {
			action.Select(c, diffTypeLabel, &t.pick, diffOptions(), action.SelectOptions{ID: "diff-type"})
		}
		design.Spacer(c)
		if t.err != nil {
			design.Text(c, decideErrorTitle+": "+t.err.Error(), token.TextLabel).SingleLine().Shrink(1).MinWidth(0).TextColor(p.Severity.Error)
		}
		if t.done {
			return
		}
		close, _ := action.Lookup(CloseID)
		if action.Button(c, closeLabel, action.ButtonOptions{Keys: close.Keys, ID: CloseID}).Clicked() {
			t.Dismiss()
		}
		t.decision(c)
	})
}

func (t *Tab) subject() string {
	switch {
	case t.req.PR != nil:
		if t.pr != nil {
			if pr := t.pr.PR(); pr.Title != "" {
				return fmt.Sprintf("%s#%d %s", pr.Repo, pr.Number, pr.Title)
			}
		}
		return t.req.PR.Display() + "#" + strconv.Itoa(t.req.PR.Number)
	case t.req.Kind == review.KindReview:
		base := ""
		if t.changes != nil {
			base = t.changes.Base()
		}
		return review.DescribeDiff(t.mode, base)
	case t.req.Kind == review.KindPlan:
		return planTitle
	}
	return documentName(t.req)
}

func diffOptions() []action.Option[review.DiffMode] {
	out := make([]action.Option[review.DiffMode], len(review.DiffModes))
	for i, m := range review.DiffModes {
		out[i] = action.Option[review.DiffMode]{Value: m, Label: m.Title()}
	}
	return out
}

func (t *Tab) decision(c *ui.Context) {
	n := len(t.notes)
	decide, _ := action.Lookup(DecideID)
	switch {
	case !t.req.CanApprove():
		if action.Button(c, sendLabel, action.ButtonOptions{Variant: action.Primary, Keys: decide.Keys, Disabled: n == 0, ID: DecideID}).Clicked() {
			t.Decide()
		}
	case n == 0:
		if action.Button(c, approveLabel, action.ButtonOptions{Variant: action.Primary, Keys: decide.Keys, ID: DecideID}).Clicked() {
			t.Decide()
		}
	default:
		menu := func(m *ui.Menu) {
			if t.req.Notes {
				notes, _ := action.Lookup(ApproveWithNotesID)
				if action.Item(m, approveNotesLabel, action.ItemOptions{Detail: notes.Keys, ID: ApproveWithNotesID}).Chosen() {
					t.ApproveWithNotes()
				}
			}
			if action.Item(m, discardLabel(n), action.ItemOptions{ID: "approve-discard"}).Chosen() {
				t.ask(confirmDiscard)
			}
		}
		if action.SplitButton(c, sendLabel, action.SplitOptions{Variant: action.Primary, Keys: decide.Keys, ID: DecideID, MenuLabel: decisionsMenuLabel, Menu: menu}).Clicked() {
			t.Decide()
		}
	}
}

func discardLabel(n int) string {
	return "Approve, Discard " + countOf(n, "Annotation")
}

func countOf(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

func (t *Tab) confirmDialog(c *ui.Context) {
	if t.confirm == confirmNone {
		return
	}
	n := len(t.notes)
	var title, hint, ok string
	var run func()
	switch t.confirm {
	case confirmClose:
		title, hint, ok = "Annotations Won't Be Sent", fmt.Sprintf("Closing the review discards %s. The agent gets no feedback.", countOf(n, annotationNoun)), "Close Review"
		run = func() { t.finish(review.Dismissed, nil) }
	case confirmDiscard:
		title, hint, ok = "Approve and Discard Annotations?", fmt.Sprintf("The agent gets an approval without your %s.", countOf(n, annotationNoun)), approveLabel
		run = func() { t.finish(review.Approved, nil) }
	case confirmSwitch:
		title, hint, ok = "Switch the Diff?", fmt.Sprintf("Switching to %s discards %s.", t.pick.Title(), countOf(n, annotationNoun)), "Switch"
		run = t.switchMode
	}
	dialog.Modal(c, &t.dialog, title, dialog.Options{Hint: hint}, func() {
		dialog.Actions(c, func() {
			if action.Button(c, cancelLabel, action.ButtonOptions{ID: "confirm-cancel"}).Clicked() {
				t.dialog = false
			}
			if action.Button(c, ok, action.ButtonOptions{Variant: action.Primary, ID: "confirm-ok"}).Clicked() {
				t.dialog = false
				run()
			}
		})
	})
	if !t.dialog {
		if t.confirm == confirmSwitch {
			t.pick = t.mode
		}
		t.confirm = confirmNone
	}
}

func (t *Tab) final(c *ui.Context) {
	o := feedback.EmptyOptions{Icon: icons.Check}
	switch t.result {
	case review.Annotated:
		o.Title, o.Message = "Feedback Sent", "The agent received your feedback."
	case review.Approved:
		o.Title, o.Message = "Changes Approved", "The agent received your approval."
	default:
		o.Icon, o.Title, o.Message = icons.Close, "Review Closed", "The agent was told that the review closed without feedback."
	}
	if t.gone {
		o.Icon, o.Message = icons.Warning, "The agent stopped waiting, so it did not receive this decision."
	}
	o.Message += " You can close this tab."
	ui.Column(c).Fill().Children(func() { feedback.Empty(c, o) })
}
