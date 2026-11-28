package review

import "testing"

const fence = "```"

func TestCodeFeedbackMatchesPlannotatorFormat(t *testing.T) {
	t.Parallel()
	anns := []Annotation{
		{Scope: ScopeGeneral, Text: "Looks good overall."},
		{Scope: ScopeLine, Path: "a.go", Side: SideNew, Start: 12, End: 12, Text: "Rename this.", Quote: "x := y"},
		{Scope: ScopeFile, Path: "a.go", Text: "Split this file."},
		{Scope: ScopeLine, Path: "b.go", Side: SideOld, Start: 3, End: 5, Text: "Simplify.", Quote: "x := 1", Suggestion: "x := 2"},
	}
	want := "# Code Review Feedback\n\n**Diff:** Uncommitted changes\n\n" +
		"## Global comments\n\nLooks good overall.\n\n" +
		"## a.go\n\n### File Comment\nSplit this file.\n\n### Line 12 (new)\nRename this.\n\n" +
		"## b.go\n\n### Lines 3-5 (old)\nSimplify.\n\n**Replaces:**\n" + fence + "\nx := 1\n" + fence + "\n\n**Suggested code:**\n" + fence + "\nx := 2\n" + fence + "\n\n"
	if got := CodeFeedback(anns, DescribeDiff(DiffUncommitted, ""), nil); got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestCodeFeedbackForAPullRequestAndForNoAnnotations(t *testing.T) {
	t.Parallel()
	pr := &PR{Repo: "o/r", Number: 7, Title: "Fix it", Head: "fix", Base: "main", URL: "https://github.com/o/r/pull/7"}
	anns := []Annotation{{Scope: ScopeLine, Path: "a.go", Side: SideNew, Start: 1, End: 1, Text: "Why?"}}
	want := "# Pull Request Review: o/r#7\n\n**Fix it**\nBranch: `fix` -> `main`\nhttps://github.com/o/r/pull/7\n\n## a.go\n\n### Line 1 (new)\nWhy?\n\n"
	if got := CodeFeedback(anns, "ignored", pr); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if got := CodeFeedback(nil, "", nil); got != "# Code Review\n\nNo feedback provided." {
		t.Fatalf("empty feedback %q", got)
	}
}

func TestFenceIsLongerThanEveryBacktickRun(t *testing.T) {
	t.Parallel()
	for code, want := range map[string]string{"plain": fence, "a ```` b": "`````", "`x`": fence} {
		if got := Fence(code); got != want {
			t.Errorf("Fence(%q) = %q, want %q", code, got, want)
		}
	}
}

func TestDescribeDiffLabels(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		mode DiffMode
		base string
		want string
	}{
		{DiffUncommitted, "", "Uncommitted changes"}, {DiffStaged, "", "Staged changes"}, {DiffUnstaged, "", "Unstaged changes"},
		{DiffLastCommit, "", "Last commit"}, {DiffBranch, "main", "Branch diff vs `main`"},
	} {
		if got := DescribeDiff(c.mode, c.base); got != c.want {
			t.Errorf("DescribeDiff(%s) = %q, want %q", c.mode, got, c.want)
		}
	}
}

func TestDocFeedbackNumbersEntriesInDocumentOrder(t *testing.T) {
	t.Parallel()
	anns := []Annotation{
		{Scope: ScopeLine, Start: 12, End: 12, Quote: "Quote text", Text: "Use a queue."},
		{Scope: ScopeGeneral, Text: "Add tests."},
		{Scope: ScopeLine, Start: 3, End: 4, Quote: "a\nb", Suggestion: "c"},
	}
	want := "# Plan Feedback\n\nI've reviewed this plan and have 3 pieces of feedback:\n\n" +
		"## 1. General feedback about the plan\n> Add tests.\n\n" +
		"## 2. (lines 3-4) Feedback on: \"a\nb\"\n\n**Replaces:**\n" + fence + "\na\nb\n" + fence + "\n\n**Suggested code:**\n" + fence + "\nc\n" + fence + "\n\n" +
		"## 3. (line 12) Feedback on: \"Quote text\"\n> Use a queue.\n\n---\n"
	if got := DocFeedback(anns, DocPlan); got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
	one := DocFeedback(anns[1:2], DocFile)
	if one != "# File Feedback\n\nI've reviewed this document and have 1 piece of feedback:\n\n## 1. General feedback about the document\n> Add tests.\n\n---\n" {
		t.Fatalf("one annotation: %q", one)
	}
}
