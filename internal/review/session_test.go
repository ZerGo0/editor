package review

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSessionRoundTrip(t *testing.T) {
	t.Parallel()
	userData := t.TempDir()
	s, err := NewSession(userData)
	if err != nil {
		t.Fatal(err)
	}
	req := Request{Kind: KindReview, Folder: "/repo", PID: os.Getpid(), Diff: DiffBranch, Base: "main", Notes: true}
	if err := s.Write(req); err != nil {
		t.Fatal(err)
	}
	opened, got, err := OpenRequest(userData, s.RequestPath())
	if err != nil || got != req || opened.Dir != s.Dir {
		t.Fatalf("OpenRequest = %+v, %+v, %v", opened, got, err)
	}
	want := Result{Decision: Annotated, Diff: DiffBranch, Base: "main", Annotations: []Annotation{lineNote}}
	if err := opened.Decide(got, want); err != nil {
		t.Fatal(err)
	}
	res, err := s.Wait(context.Background(), time.Millisecond)
	if err != nil || res.Decision != want.Decision || len(res.Annotations) != 1 || res.Annotations[0] != lineNote || res.Base != "main" {
		t.Fatalf("Wait = %+v, %v", res, err)
	}
	if err := s.Remove(); err != nil {
		t.Fatal(err)
	}
	if err := opened.Decide(got, want); !errors.Is(err, ErrAgentGone) {
		t.Fatalf("Decide after Remove = %v, want ErrAgentGone", err)
	}
}

func TestOpenRequestRejectsFilesOutsideTheSessions(t *testing.T) {
	t.Parallel()
	userData := t.TempDir()
	other := filepath.Join(t.TempDir(), requestName)
	if err := os.WriteFile(other, []byte("{}"), filePerm); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{other, filepath.Join(userData, DirName, "x", "other.json"), filepath.Join(userData, DirName, "..", requestName)} {
		if _, _, err := OpenRequest(userData, p); err == nil {
			t.Errorf("OpenRequest(%s) accepted the file", p)
		}
	}
}

func TestDecideFailsWhenTheAgentIsGone(t *testing.T) {
	t.Parallel()
	s, err := NewSession(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Decide(Request{PID: 0}, Result{Decision: Approved}); !errors.Is(err, ErrAgentGone) {
		t.Fatalf("Decide = %v, want ErrAgentGone", err)
	}
}

func TestNewSessionPrunesSessionsOfStoppedAgents(t *testing.T) {
	t.Parallel()
	userData := t.TempDir()
	live, _ := NewSession(userData)
	dead, _ := NewSession(userData)
	if err := live.Write(Request{Kind: KindPlan, Folder: "/p", Text: "x", PID: os.Getpid()}); err != nil {
		t.Fatal(err)
	}
	if err := dead.Write(Request{Kind: KindPlan, Folder: "/p", Text: "x", PID: 0}); err != nil {
		t.Fatal(err)
	}
	if _, err := NewSession(userData); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(live.Dir); err != nil {
		t.Errorf("live session removed: %v", err)
	}
	if _, err := os.Stat(dead.Dir); !os.IsNotExist(err) {
		t.Errorf("dead session kept: %v", err)
	}
}

func TestRequestValidate(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		req Request
		ok  bool
	}{
		{Request{Kind: KindReview, Folder: "/r", Diff: DiffStaged}, true},
		{Request{Kind: KindReview, Folder: "/r", Diff: "nope"}, false},
		{Request{Kind: KindReview, Folder: "rel", Diff: DiffStaged}, false},
		{Request{Kind: KindReview, Folder: "/r", PR: &PRRef{Owner: "o", Repo: "r", Number: 1}}, true},
		{Request{Kind: KindReview, Folder: "/r", PR: &PRRef{Owner: "o"}}, false},
		{Request{Kind: KindAnnotate, Folder: "/r", File: "/r/a.md"}, true},
		{Request{Kind: KindAnnotate, Folder: "/r", File: "a.md"}, false},
		{Request{Kind: KindPlan, Folder: "/r"}, false},
		{Request{Kind: "other", Folder: "/r"}, false},
	} {
		if err := c.req.Validate(); (err == nil) != c.ok {
			t.Errorf("Validate(%+v) = %v", c.req, err)
		}
	}
}

func TestParsePRURL(t *testing.T) {
	t.Parallel()
	got, ok := ParsePRURL("https://github.com/o/r/pull/12/files")
	if !ok || got != (PRRef{Host: "github.com", Owner: "o", Repo: "r", Number: 12, URL: "https://github.com/o/r/pull/12/files"}) {
		t.Fatalf("ParsePRURL = %+v, %v", got, ok)
	}
	for _, s := range []string{"", ".", "github.com/o/r/pull/1", "https://github.com/o/r/issues/1", "https://github.com/o/r/pull/x", "ftp://h/o/r/pull/1"} {
		if _, ok := ParsePRURL(s); ok {
			t.Errorf("ParsePRURL(%q) accepted", s)
		}
	}
}
