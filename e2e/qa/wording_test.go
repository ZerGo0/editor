package qa

import "testing"

func issueRules(issues []Issue) []string {
	var out []string
	for _, i := range issues {
		out = append(out, i.Rule)
	}
	return out
}

func TestCheckTitleAcceptsVSCodeTitles(t *testing.T) {
	for _, title := range []string{
		"Go to File...", "Go to Line/Column...", "Close All Editors in Group", "Open Preview to the Side",
		"Show All Commands", "Open Settings (UI)", "Install 'editor' Command in PATH", "Reveal Active File in Finder",
		"Find in Files", "Copy Path of Active File", "Open Keyboard Shortcuts (JSON)", "Compare with Saved",
		"Zoom In", "Import VS Code Keybindings...", "Add Current File to .gitignore",
	} {
		if issues := CheckTitle(title, false, false); len(issues) != 0 {
			t.Errorf("%q: %v", title, issues)
		}
	}
}

func TestCheckTitleFlagsCaseSpacingAndPunctuation(t *testing.T) {
	cases := []struct {
		title string
		rule  string
	}{
		{"Toggle sidebar", RuleTitleCase},
		{"Go To File...", RuleTitleCase},
		{"Open  File", RuleDoubleSpace},
		{"Open File ", RuleEdgeSpace},
		{"Save.", RuleTrailingPunct},
		{"Open File\u2026", RuleEllipsis},
	}
	for _, c := range cases {
		issues := CheckTitle(c.title, false, false)
		found := false
		for _, i := range issues {
			found = found || i.Rule == c.rule
		}
		if !found {
			t.Errorf("%q: rules %v, want %s", c.title, issueRules(issues), c.rule)
		}
	}
}

func TestCheckTitleEllipsisFollowsInput(t *testing.T) {
	if issues := CheckTitle("Open Folder", true, true); len(issues) != 1 || issues[0].Rule != RuleEllipsis {
		t.Fatalf("issues = %v", issues)
	}
	if issues := CheckTitle("Save...", false, true); len(issues) != 1 || issues[0].Rule != RuleEllipsis {
		t.Fatalf("issues = %v", issues)
	}
	if issues := CheckTitle("Open Folder...", true, true); len(issues) != 0 {
		t.Fatalf("issues = %v", issues)
	}
	if issues := CheckTitle("Open Folder", true, false); len(issues) != 0 {
		t.Fatalf("unclassified flagged: %v", issues)
	}
}

func TestCheckVSCodeWords(t *testing.T) {
	if issues := CheckVSCodeWords("Shelve Changes..."); len(issues) != 1 || issues[0].Rule != RuleForeignWord {
		t.Fatalf("issues = %v", issues)
	}
	if issues := CheckVSCodeWords("Stash Changes..."); len(issues) != 0 {
		t.Fatalf("issues = %v", issues)
	}
}

func TestCompareVSCode(t *testing.T) {
	if issue := CompareVSCode("workbench.action.files.save", "Save"); issue != nil {
		t.Fatalf("issue = %v", issue)
	}
	if issue := CompareVSCode("workbench.action.files.newUntitledFile", "New Text File"); issue != nil {
		t.Fatalf("menu variant flagged: %v", issue)
	}
	if issue := CompareVSCode("workbench.action.files.save", "Write"); issue == nil || issue.Rule != RuleVSCodeWording {
		t.Fatalf("issue = %v", issue)
	}
	if issue := CompareVSCode("unknown.command", "Anything"); issue != nil {
		t.Fatalf("unknown flagged: %v", issue)
	}
}

func TestVSCodeTablesAreConsistent(t *testing.T) {
	for id, titles := range VSCodeTitles {
		if len(titles) == 0 {
			t.Errorf("%s has no titles", id)
		}
		for _, title := range titles {
			if title != "" && len(CheckTitle(title, false, false)) != 0 && id != "workbench.action.openSettings" {
				t.Logf("%s: VS Code title %q breaks our rules: %v", id, title, CheckTitle(title, false, false))
			}
		}
	}
}
