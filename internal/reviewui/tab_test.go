package reviewui

import (
	"context"
	"testing"
	"time"

	"github.com/ZerGo0/editor/internal/review"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	testWidth    = 1200
	testHeight   = 800
	settleLimit  = 5 * time.Second
	planText     = "# Plan\n\nStep one.\nStep two.\n"
	gutterInset  = 4
	newTextLabel = "New text"
)

type rig struct {
	t       *testing.T
	tab     *Tab
	tt      *ui.Tester
	posted  chan func()
	results []review.Result
}

func newRig(t *testing.T, req review.Request) *rig {
	t.Helper()
	r := &rig{t: t, posted: make(chan func(), 64)}
	r.tab = New(req, Deps{
		Ctx:       context.Background(),
		Post:      func(fn func()) { r.posted <- fn },
		Decide:    func(res review.Result) error { r.results = append(r.results, res); return nil },
		Configure: func(*editor.DiffState, string) {},
		Language:  func(string, string) *editor.Language { return nil },
		ReadFile:  func(string) (string, error) { return planText, nil },
	})
	t.Cleanup(r.tab.Close)
	r.tt = ui.NewTester(func(c *ui.Context) {
		for {
			select {
			case fn := <-r.posted:
				fn()
				continue
			default:
			}
			break
		}
		r.tab.View(c)
	}, testWidth, testHeight)
	r.until("the document", func() bool { _, ok := r.tt.Access(diffLabel(req)); return ok })
	return r
}

func (r *rig) until(what string, ok func() bool) {
	r.t.Helper()
	deadline := time.After(settleLimit)
	for {
		r.tt.Frame()
		if ok() {
			return
		}
		if r.tt.FramePending() {
			continue
		}
		select {
		case fn := <-r.posted:
			fn()
		case <-deadline:
			r.t.Fatalf("waiting for %s; texts: %v", what, r.tt.Texts())
		}
	}
}

func (r *rig) click(label string) {
	r.t.Helper()
	if err := r.tt.Click(label); err != nil {
		r.t.Fatalf("click %q: %v; texts: %v", label, err, r.tt.Texts())
	}
	r.tt.Frame()
}

func (r *rig) addGlobalComment(text string) {
	r.t.Helper()
	r.click(addGlobalLabel)
	r.tt.Type(text)
	r.tt.Frame()
	r.click(addCommentLabel)
	r.until("the global comment", func() bool { return r.tt.HasText(text) && !r.tab.panel.open })
}

func diffLabel(req review.Request) string { return "Diff of " + documentName(req) }

func planRequest() review.Request {
	return review.Request{Kind: review.KindPlan, Folder: "/p", Text: planText}
}

func TestInlineComposerAddsALineAnnotation(t *testing.T) {
	t.Parallel()
	r := newRig(t, planRequest())
	text, ok := r.tt.Access(newTextLabel)
	if !ok {
		t.Fatalf("no diff text; texts: %v", r.tt.Texts())
	}
	x, y := text.Bounds.X+gutterInset, text.Bounds.Y+gutterInset
	r.tt.Press(x, y)
	r.tt.Release(x, y)
	r.until("the composer", func() bool { return r.tt.HasText(review.LineLabel(1, 1)) })
	if !r.tt.HasText(commentLabel) || !r.tt.HasText(addSuggestionLabel) {
		t.Fatalf("composer has no comment field; texts: %v", r.tt.Texts())
	}
	r.tt.Type("Name the goal.")
	r.tt.Frame()
	r.tt.Key(ui.Cmd, ui.KeyEnter)
	r.until("the inline card", func() bool { return len(r.tab.notes) == 1 && !r.tab.inline.open })
	got := r.tab.notes[0].Annotation
	want := review.Annotation{Scope: review.ScopeLine, Path: planFileName, Side: review.SideNew, Start: 1, End: 1, Text: "Name the goal.", Quote: "# Plan"}
	if got != want {
		t.Fatalf("annotation = %+v, want %+v", got, want)
	}
	if !r.tt.HasText("L1") || !r.tt.HasText(sendLabel) {
		t.Fatalf("no card or Send Feedback; texts: %v", r.tt.Texts())
	}
}

func TestDecisionSwitchesFromApproveToSendFeedback(t *testing.T) {
	t.Parallel()
	r := newRig(t, planRequest())
	if !r.tt.HasText(approveLabel) || r.tt.HasText(sendLabel) {
		t.Fatalf("want Approve without annotations; texts: %v", r.tt.Texts())
	}
	r.addGlobalComment("Add a rollback step.")
	if !r.tt.HasText(sendLabel) {
		t.Fatalf("want Send Feedback with an annotation; texts: %v", r.tt.Texts())
	}
	r.click(sendLabel)
	if len(r.results) != 1 || r.results[0].Decision != review.Annotated || len(r.results[0].Annotations) != 1 {
		t.Fatalf("results = %+v", r.results)
	}
	if r.results[0].Annotations[0] != (review.Annotation{Scope: review.ScopeGeneral, Text: "Add a rollback step."}) {
		t.Fatalf("annotation = %+v", r.results[0].Annotations[0])
	}
	r.until("the final state", func() bool { return r.tt.HasText("Feedback Sent") })
}

func TestAnnotateWithoutGateOffersNoApproval(t *testing.T) {
	t.Parallel()
	r := newRig(t, review.Request{Kind: review.KindAnnotate, Folder: "/p", File: "/p/notes.md"})
	if r.tt.HasText(approveLabel) {
		t.Fatalf("Approve shown without -gate; texts: %v", r.tt.Texts())
	}
	r.click(sendLabel)
	if len(r.results) != 0 {
		t.Fatalf("Send Feedback without annotations decided: %+v", r.results)
	}
}

func TestApproveWithNotesKeepsTheAnnotations(t *testing.T) {
	t.Parallel()
	r := newRig(t, review.Request{Kind: review.KindAnnotate, Folder: "/p", File: "/p/notes.md", Gate: true, Notes: true})
	r.addGlobalComment("Fine, but shorten it later.")
	r.click(decisionsMenuLabel)
	r.click(approveNotesLabel)
	if len(r.results) != 1 || r.results[0].Decision != review.Approved || len(r.results[0].Annotations) != 1 {
		t.Fatalf("results = %+v", r.results)
	}
	r.until("the final state", func() bool { return r.tt.HasText("Changes Approved") })
}

func TestCloseWithAnnotationsAsksFirst(t *testing.T) {
	t.Parallel()
	r := newRig(t, planRequest())
	r.addGlobalComment("Drop step two.")
	r.click(closeLabel)
	r.until("the confirmation", func() bool { return r.tt.HasText("Annotations Won't Be Sent") })
	r.click(cancelLabel)
	if len(r.results) != 0 || r.tt.HasText("Annotations Won't Be Sent") {
		t.Fatalf("Cancel decided or kept the dialog: %+v", r.results)
	}
	r.click(closeLabel)
	r.click("Close Review")
	if len(r.results) != 1 || r.results[0].Decision != review.Dismissed || len(r.results[0].Annotations) != 0 {
		t.Fatalf("results = %+v", r.results)
	}
	r.until("the final state", func() bool { return r.tt.HasText("Review Closed") })
}
