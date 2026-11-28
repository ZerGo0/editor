//go:build e2e

package e2e

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const reviewCLITimeout = 30 * time.Second

func TestReviewSendsFeedbackToTheWaitingCLI(t *testing.T) {
	t.Parallel()
	root := gitProject(t, map[string]string{"main.go": "package main\n"})
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	app := Launch(t, root)
	win := app.WaitWindow(filepath.Base(root))
	app.Idle(win)

	var stdout, stderr bytes.Buffer
	cmd := exec.Command(appBinary, "review")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), socketEnv+"=", userDataEnv+"="+app.UserData, runtimeDirEnv+"="+app.runtimeDir)
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	t.Cleanup(func() { _ = cmd.Process.Kill() })

	app.WaitNode(win, "the review tab", RoleLabel("tab", "Review: "+filepath.Base(root)))
	app.WaitNode(win, "the changed file", Label("Diff of main.go"))
	app.Click(win, app.WaitNode(win, "Add Global Comment", RoleLabel("button", "Add Global Comment")), 1)
	app.Type(win, "Explain why main is empty.")
	app.Key(win, "Cmd+Enter")
	app.WaitNode(win, "the saved comment", RoleLabel("button", "Send Feedback"))
	select {
	case err := <-done:
		t.Fatalf("saving a comment ended the review: %v\n%s", err, stdout.String())
	default:
	}
	app.Key(win, "Cmd+Enter")

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("editor review: %v\n%s", err, stderr.String())
		}
	case <-time.After(reviewCLITimeout):
		t.Fatalf("editor review did not finish\nstdout: %s\nstderr: %s", stdout.String(), stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{"# Code Review Feedback", "**Diff:** Uncommitted changes", "## Global comments", "Explain why main is empty.", "Do not change any code until we have discussed the verdicts"} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout lacks %q:\n%s", want, out)
		}
	}
	app.WaitNode(win, "the final state", Label("Feedback Sent"))
}
