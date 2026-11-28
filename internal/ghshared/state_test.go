package ghshared_test

import (
	"errors"
	"testing"

	"github.com/ZerGo0/editor/internal/ghshared"
	"github.com/ZerGo0/editor/internal/ghshared/ghtest"
	"github.com/egoist/mygo/ui"
)

var testWording = ghshared.StateWording{NoRepoDetail: "Open a folder that is a Git repository to see its things.", ErrorTitle: "Cannot load things"}

func stateTester(s ghshared.Session[int], host string, connect func()) *ui.Tester {
	return ui.NewTester(func(c *ui.Context) { ghshared.StateView(c, s.StateMessage(host), connect) }, ghtest.SideWidth, ghtest.SideHeight)
}

func TestStateMessages(t *testing.T) {
	cases := []struct {
		name  string
		phase ghshared.Phase
		host  string
		err   error
		texts []string
	}{
		{"no repo", ghshared.PhaseNoRepo, "", nil, []string{"No Git repository", testWording.NoRepoDetail}},
		{"no remote", ghshared.PhaseNoRemote, "", nil, []string{"No GitHub remote", "git remote add origin"}},
		{"no gh", ghshared.PhaseNoGH, "", nil, []string{"GitHub CLI not found", "Install the GitHub CLI and run gh auth login"}},
		{"not logged in", ghshared.PhaseNotLoggedIn, "github.com", nil, []string{"Sign in to GitHub", "gh auth login", "Copy Command"}},
		{"not logged in on a host", ghshared.PhaseNotLoggedIn, "ghe.example.com", nil, []string{"gh auth login --hostname ghe.example.com"}},
		{"error", ghshared.PhaseError, "", errors.New("boom"), []string{"Cannot load things", "Boom.", "Retry"}},
		{"error without detail", ghshared.PhaseError, "", nil, []string{"Cannot load things", "Something went wrong.", "Retry"}},
		{"connecting", ghshared.PhaseLoading, "", nil, []string{"Connecting to GitHub..."}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := ghshared.Session[int]{Phase: tc.phase, PhaseErr: tc.err, Wording: testWording}
			ghtest.MustHaveText(t, stateTester(s, tc.host, func() {}), tc.texts...)
		})
	}
}

func TestConnectingSpinnerHasAccessibleName(t *testing.T) {
	tt := stateTester(ghshared.Session[int]{Wording: testWording}, "", func() {})
	if _, ok := tt.AccessByRole(ui.RoleProgress, "Connecting to GitHub..."); !ok {
		t.Error("the spinner has no accessible name")
	}
}

func TestStateViewRetryRunsConnect(t *testing.T) {
	connects := 0
	tt := stateTester(ghshared.Session[int]{Phase: ghshared.PhaseNoRepo, Wording: testWording}, "", func() { connects++ })
	ghtest.MustClick(t, tt, "Check Again")
	if connects != 1 {
		t.Errorf("connect ran %d times, want 1", connects)
	}
}
