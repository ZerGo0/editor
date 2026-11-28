package review

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type cliRun struct {
	code   int
	stdout string
	stderr string
	req    Request
}

func runCLI(t *testing.T, name string, args []string, wd, stdin string, decide func(Request) Result) cliRun {
	t.Helper()
	userData := t.TempDir()
	var out, errOut bytes.Buffer
	var seen Request
	env := Env{
		Ctx: context.Background(), UserData: userData, PID: os.Getpid(),
		Getwd: func() (string, error) { return wd, nil },
		Launch: func(path string) error {
			s, req, err := OpenRequest(userData, path)
			if err != nil {
				return err
			}
			seen = req
			return s.Decide(req, decide(req))
		},
		Stdin: strings.NewReader(stdin), Stdout: &out, Stderr: &errOut,
	}
	code := Run(name, args, env)
	if entries, _ := os.ReadDir(filepath.Join(userData, DirName)); len(entries) != 0 {
		t.Errorf("%s left %d session folders", name, len(entries))
	}
	return cliRun{code: code, stdout: out.String(), stderr: errOut.String(), req: seen}
}

func gitFolder(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, gitDir), dirPerm); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "sub"), dirPerm); err != nil {
		t.Fatal(err)
	}
	return root
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	s, err := jsonLine(v)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func decided(d Decision, anns ...Annotation) func(Request) Result {
	return func(req Request) Result {
		return Result{Decision: d, Annotations: anns, Diff: req.Diff, Base: req.Base}
	}
}

func TestRunRejectsBadUsageWithExitTwo(t *testing.T) {
	t.Parallel()
	root := gitFolder(t)
	for _, c := range []struct {
		name string
		args []string
	}{
		{CommandReview, []string{"-diff", "nope"}},
		{CommandReview, []string{"-base", "main"}},
		{CommandReview, []string{"-unknown"}},
		{CommandReview, []string{"a", "b"}},
		{CommandReview, []string{"-diff", "staged", "https://github.com/o/r/pull/1"}},
		{CommandAnnotate, nil},
		{CommandAnnotate, []string{"a.md", "b.md"}},
		{CommandPlanHook, []string{"extra"}},
	} {
		r := runCLI(t, c.name, c.args, root, "", decided(Dismissed))
		if r.code != ExitUsage || r.req.Kind != "" {
			t.Errorf("%s %v: exit %d, launched %v", c.name, c.args, r.code, r.req.Kind != "")
		}
	}
}

func TestRunFailsWithExitOneOutsideAGitRepositoryAndOnBadHookInput(t *testing.T) {
	t.Parallel()
	plain, _ := filepath.EvalSymlinks(t.TempDir())
	if r := runCLI(t, CommandReview, nil, plain, "", decided(Dismissed)); r.code != ExitFailure || !strings.Contains(r.stderr, "not in a Git repository") {
		t.Errorf("review outside git: exit %d, stderr %q", r.code, r.stderr)
	}
	for _, in := range []string{"", "{", `{"tool_input":{}}`} {
		if r := runCLI(t, CommandPlanHook, nil, plain, in, decided(Dismissed)); r.code != ExitFailure {
			t.Errorf("plan-hook %q: exit %d", in, r.code)
		}
	}
}

func TestRunReviewSendsTheRequestAndPrintsTheFeedback(t *testing.T) {
	t.Parallel()
	root := gitFolder(t)
	r := runCLI(t, CommandReview, []string{"-diff", "branch", "-base", "dev", "sub"}, root, "", decided(Annotated, lineNote))
	want := Request{Kind: KindReview, Folder: root, PID: os.Getpid(), Diff: DiffBranch, Base: "dev", Notes: true}
	if r.code != 0 || r.req != want {
		t.Fatalf("exit %d, request %+v, stderr %q", r.code, r.req, r.stderr)
	}
	if !strings.HasPrefix(r.stdout, "# Code Review Feedback\n\n**Diff:** Branch diff vs `dev`\n") || !strings.HasSuffix(r.stdout, ReviewDeniedSuffix+"\n") {
		t.Fatalf("stdout %q", r.stdout)
	}
	pr := runCLI(t, CommandReview, []string{"-json", "https://github.com/o/r/pull/3"}, filepath.Join(root, "sub"), "", decided(Approved))
	if pr.code != 0 || pr.req.PR == nil || pr.req.PR.Number != 3 || pr.req.Folder != root || pr.stdout != mustJSON(t, reviewJSON{Decision: Approved, Message: ReviewApprovedPrompt}) {
		t.Fatalf("pr: exit %d, request %+v, stdout %q", pr.code, pr.req, pr.stdout)
	}
}

func TestRunAnnotateOpensTheFile(t *testing.T) {
	t.Parallel()
	root := gitFolder(t)
	path := filepath.Join(root, "sub", "notes.md")
	if err := os.WriteFile(path, []byte("# Notes\n"), filePerm); err != nil {
		t.Fatal(err)
	}
	r := runCLI(t, CommandAnnotate, []string{"-gate", "sub/notes.md"}, root, "", decided(Approved))
	if r.code != 0 || r.req != (Request{Kind: KindAnnotate, Folder: root, PID: os.Getpid(), File: path, Gate: true, Notes: true}) || r.stdout != AnnotateApprovedMessage+"\n" {
		t.Fatalf("exit %d, request %+v, stdout %q", r.code, r.req, r.stdout)
	}
	if !r.req.CanApprove() || (Request{Kind: KindAnnotate}).CanApprove() {
		t.Fatal("only a gated annotation can be approved")
	}
	if r := runCLI(t, CommandAnnotate, []string{"sub"}, root, "", decided(Dismissed)); r.code != ExitFailure {
		t.Fatalf("annotate a folder: exit %d", r.code)
	}
}

func TestRunPlanHookUsesTheHookInput(t *testing.T) {
	t.Parallel()
	root := gitFolder(t)
	in := `{"cwd":"` + filepath.Join(root, "sub") + `","tool_input":{"plan":"# P","planFilePath":"/x/p.md"}}`
	r := runCLI(t, CommandPlanHook, nil, "/", in, decided(Dismissed))
	if r.code != 0 || r.stdout != "" || r.req != (Request{Kind: KindPlan, Folder: root, PID: os.Getpid(), Text: "# P", File: "/x/p.md"}) {
		t.Fatalf("exit %d, request %+v, stdout %q, stderr %q", r.code, r.req, r.stdout, r.stderr)
	}
}
