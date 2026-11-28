package review

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
)

const (
	noCodeFeedback        = "# Code Review\n\nNo feedback provided."
	noDocFeedback         = "No changes detected."
	codeFeedbackTitle     = "# Code Review Feedback\n\n"
	globalCommentsHeading = "## Global comments\n\n"
	fileCommentHeading    = "File Comment"
	replacesHead          = "**Replaces:**"
	suggestedHead         = "**Suggested code:**"
	minFence              = 3
	fenceRune             = '`'
)

type PR struct {
	Repo   string `json:"repo"`
	Number int    `json:"number"`
	Title  string `json:"title"`
	Head   string `json:"head"`
	Base   string `json:"base"`
	URL    string `json:"url"`
}

type DocKind uint8

const (
	DocPlan DocKind = iota
	DocFile
)

func CodeFeedback(anns []Annotation, diffLabel string, pr *PR) string {
	if len(anns) == 0 {
		return noCodeFeedback
	}
	var b strings.Builder
	if pr != nil {
		fmt.Fprintf(&b, "# Pull Request Review: %s#%d\n\n**%s**\nBranch: `%s` -> `%s`\n%s\n\n", pr.Repo, pr.Number, pr.Title, pr.Head, pr.Base, pr.URL)
	} else {
		b.WriteString(codeFeedbackTitle)
		if diffLabel != "" {
			fmt.Fprintf(&b, "**Diff:** %s\n\n", diffLabel)
		}
	}
	var general, placed []Annotation
	for _, a := range anns {
		if a.Scope == ScopeGeneral {
			general = append(general, a)
		} else {
			placed = append(placed, a)
		}
	}
	if len(general) > 0 {
		b.WriteString(globalCommentsHeading)
		for _, a := range general {
			writeText(&b, a.Text)
			b.WriteString("\n")
		}
	}
	var paths []string
	byPath := map[string][]Annotation{}
	for _, a := range placed {
		if _, ok := byPath[a.Path]; !ok {
			paths = append(paths, a.Path)
		}
		byPath[a.Path] = append(byPath[a.Path], a)
	}
	for _, path := range paths {
		fmt.Fprintf(&b, "## %s\n\n", path)
		for _, a := range sortedInFile(byPath[path]) {
			if a.Scope == ScopeFile {
				fmt.Fprintf(&b, "### %s\n", fileCommentHeading)
			} else {
				fmt.Fprintf(&b, "### %s (%s)\n", LineLabel(a.Start, a.End), a.Side)
			}
			writeText(&b, a.Text)
			writeSuggestion(&b, a)
			b.WriteString("\n")
		}
	}
	return b.String()
}

func sortedInFile(anns []Annotation) []Annotation {
	out := slices.Clone(anns)
	slices.SortStableFunc(out, func(a, b Annotation) int {
		if (a.Scope == ScopeFile) != (b.Scope == ScopeFile) {
			if a.Scope == ScopeFile {
				return -1
			}
			return 1
		}
		return cmp.Compare(a.Start, b.Start)
	})
	return out
}

func writeText(b *strings.Builder, text string) {
	if text != "" {
		b.WriteString(text + "\n")
	}
}

func writeSuggestion(b *strings.Builder, a Annotation) {
	if a.Suggestion == "" {
		return
	}
	if a.Quote != "" {
		b.WriteString("\n" + replacesHead + "\n" + fenced(a.Quote))
	}
	b.WriteString("\n" + suggestedHead + "\n" + fenced(a.Suggestion))
}

func fenced(code string) string {
	fence := Fence(code)
	return fence + "\n" + code + "\n" + fence + "\n"
}

func Fence(code string) string {
	longest, run := 0, 0
	for _, r := range code {
		if r == fenceRune {
			run++
			longest = max(longest, run)
			continue
		}
		run = 0
	}
	return strings.Repeat(string(fenceRune), max(minFence, longest+1))
}

func DocFeedback(anns []Annotation, kind DocKind) string {
	if len(anns) == 0 {
		return noDocFeedback
	}
	title, subject := "Plan Feedback", "plan"
	if kind == DocFile {
		title, subject = "File Feedback", "document"
	}
	sorted := slices.Clone(anns)
	slices.SortStableFunc(sorted, func(a, b Annotation) int {
		if ga, gb := a.Scope != ScopeLine, b.Scope != ScopeLine; ga != gb {
			if ga {
				return -1
			}
			return 1
		}
		return cmp.Compare(a.Start, b.Start)
	})
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", title)
	pieces := "piece"
	if len(sorted) > 1 {
		pieces += "s"
	}
	fmt.Fprintf(&b, "I've reviewed this %s and have %d %s of feedback:\n\n", subject, len(sorted), pieces)
	for i, a := range sorted {
		fmt.Fprintf(&b, "## %d. ", i+1)
		if a.Scope != ScopeLine {
			fmt.Fprintf(&b, "General feedback about the %s\n", subject)
			writeQuote(&b, a.Text)
			b.WriteString("\n")
			continue
		}
		fmt.Fprintf(&b, "(%s) Feedback on: \"%s\"\n", docLineLabel(a.Start, a.End), a.Quote)
		writeQuote(&b, a.Text)
		writeSuggestion(&b, a)
		b.WriteString("\n")
	}
	b.WriteString("---\n")
	return b.String()
}

func writeQuote(b *strings.Builder, text string) {
	if text != "" {
		b.WriteString("> " + text + "\n")
	}
}

func docLineLabel(start, end int) string { return rangeLabel("line", "lines", "-", start, end) }
