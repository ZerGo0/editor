package review

import (
	"encoding/json"
	"strings"
	"testing"
)

var lineNote = Annotation{Scope: ScopeLine, Path: "a.go", Side: SideNew, Start: 2, End: 2, Text: "Fix.", Quote: "x"}

func TestReviewOutputForEachDecision(t *testing.T) {
	t.Parallel()
	feedback := "# Code Review Feedback\n\n**Diff:** Staged changes\n\n## a.go\n\n### Line 2 (new)\nFix.\n\n"
	for _, c := range []struct {
		name string
		res  Result
		want string
	}{
		{"dismissed", Result{Decision: Dismissed}, "Review session closed without feedback.\n"},
		{"approved", Result{Decision: Approved, Diff: DiffStaged}, ReviewApprovedPrompt + "\n"},
		{"notes", Result{Decision: Approved, Diff: DiffStaged, Annotations: []Annotation{lineNote}},
			"# Code Review: Approved with Notes\n\nCode review completed: the changes are approved. The notes below are non-blocking guidance, not a request for another revision.\n\n" +
				strings.TrimSpace(feedback) + "\n\nDo not revise or reopen the reviewed changes solely because of these notes unless the user explicitly requests it. Carry them into subsequent work where applicable.\n"},
		{"feedback", Result{Decision: Annotated, Diff: DiffStaged, Annotations: []Annotation{lineNote}}, feedback + "\n" + ReviewDeniedSuffix + "\n"},
	} {
		got, err := ReviewOutput(c.res, false)
		if err != nil || got != c.want {
			t.Errorf("%s: got %q, err %v\nwant %q", c.name, got, err, c.want)
		}
	}
	got, err := ReviewOutput(Result{Decision: Dismissed}, true)
	if err != nil || got != "{\"decision\":\"dismissed\",\"message\":\"Review session closed without feedback.\"}\n" {
		t.Fatalf("json: %q, %v", got, err)
	}
}

func TestAnnotateOutputFollowsTheFlags(t *testing.T) {
	t.Parallel()
	const file = "/p/notes.md"
	doc := Annotation{Scope: ScopeLine, Start: 1, End: 1, Quote: "# Title", Text: "Shorter."}
	fb := "# Markdown Annotations\n\nFile: /p/notes.md\n\n" + DocFeedback([]Annotation{doc}, DocFile) + "\n\nPlease address the annotation feedback above."
	annotated := Result{Decision: Annotated, Annotations: []Annotation{doc}}
	notes := Result{Decision: Approved, Annotations: []Annotation{doc}}
	enc := func(s string) string { out, _ := jsonLine(s); return strings.TrimSuffix(out, "\n") }
	for _, c := range []struct {
		name  string
		res   Result
		flags AnnotateFlags
		want  string
	}{
		{"plain close", Result{Decision: Dismissed}, AnnotateFlags{}, ""},
		{"plain approve", Result{Decision: Approved}, AnnotateFlags{}, "The user approved.\n"},
		{"plain notes", notes, AnnotateFlags{}, "# Approved with Notes\n\nThe artifact is approved. The notes below are non-blocking guidance, not a request for another revision.\n\nFile: /p/notes.md\n\n" +
			DocFeedback(notes.Annotations, DocFile) + "\n\nDo not revise or reopen the artifact solely because of these notes unless the user explicitly requests it. Carry the notes into subsequent work where applicable.\n"},
		{"plain feedback", annotated, AnnotateFlags{}, fb + "\n"},
		{"json close", Result{Decision: Dismissed}, AnnotateFlags{JSON: true}, "{\"decision\":\"dismissed\",\"target\":\"/p/notes.md\"}\n"},
		{"json approve", Result{Decision: Approved}, AnnotateFlags{JSON: true}, "{\"decision\":\"approved\",\"annotationCount\":0,\"target\":\"/p/notes.md\"}\n"},
		{"json feedback", annotated, AnnotateFlags{JSON: true}, "{\"decision\":\"annotated\",\"feedback\":" + enc(fb) + ",\"annotationCount\":1,\"target\":\"/p/notes.md\"}\n"},
		{"hook approve", Result{Decision: Approved}, AnnotateFlags{Hook: true}, ""},
		{"hook feedback", annotated, AnnotateFlags{Hook: true, JSON: true}, "{\"decision\":\"block\",\"reason\":" + enc(fb) + "}\n"},
	} {
		got, err := AnnotateOutput(file, c.res, c.flags)
		if err != nil || got != c.want {
			t.Errorf("%s: got %q, err %v\nwant %q", c.name, got, err, c.want)
		}
	}
}

func TestPlanHookOutputMatchesThePermissionRequestProtocol(t *testing.T) {
	t.Parallel()
	req := Request{Kind: KindPlan, Text: "# Plan\n\nStep one.", File: "/home/me/.claude/plans/p.md"}
	input := json.RawMessage(`{"plan":"old","planFilePath":"/home/me/.claude/plans/p.md"}`)
	got, err := PlanHookOutput(req, input, Result{Decision: Approved})
	want := `{"hookSpecificOutput":{"hookEventName":"PermissionRequest","decision":{"behavior":"allow","updatedInput":{"plan":"# Plan\n\nStep one.","planFilePath":"/home/me/.claude/plans/p.md"}}}}` + "\n"
	if err != nil || got != want {
		t.Fatalf("approve: %s, %v\nwant %s", got, err, want)
	}
	note := Annotation{Scope: ScopeLine, Start: 3, End: 3, Quote: "Step one.", Text: "Add a test step."}
	got, err = PlanHookOutput(req, input, Result{Decision: Annotated, Annotations: []Annotation{note}})
	if err != nil {
		t.Fatal(err)
	}
	var out permissionOutput
	if err := json.Unmarshal([]byte(got), &out); err != nil {
		t.Fatal(err)
	}
	wantMsg := "YOUR PLAN WAS NOT APPROVED.\n\nYou MUST revise the plan to address ALL of the feedback below before calling ExitPlanMode again.\n\nRules:\n" +
		"- Your plan is saved at: /home/me/.claude/plans/p.md\n  You can edit this file to make targeted changes, then pass its path to ExitPlanMode.\n" +
		"- Do not resubmit the same plan unchanged.\n- Do NOT change the plan title (first # heading) unless the user explicitly asks you to.\n\n" + DocFeedback([]Annotation{note}, DocPlan)
	if d := out.HookSpecificOutput; d.HookEventName != "PermissionRequest" || d.Decision.Behavior != "deny" || d.Decision.Message != wantMsg {
		t.Fatalf("deny: %+v\nwant message %q", d, wantMsg)
	}
	if got, err := PlanHookOutput(req, input, Result{Decision: Dismissed}); err != nil || got != "" {
		t.Fatalf("close printed %q, %v", got, err)
	}
}
