//go:build e2e

package e2e

import (
	"bytes"
	"fmt"
)

const (
	demoCommitCount     = 2100
	demoEpoch           = 1735689600
	demoCommitStep      = 25200
	demoMergeEvery      = 40
	demoSideMarkBase    = 100000
	demoVersionEvery    = 7
	demoLongSubjectEach = 97
	demoSideCommits     = 2
	demoConflictMainAt  = demoCommitCount - 4
	demoConflictBaseAt  = demoCommitCount - 8
	demoFileMode        = "644"

	demoFeatureBranch  = "feature/implement-the-extremely-long-named-feature-branch-with-many-words-and-more"
	demoBugfixBranch   = "bugfix/JIRA-12345-fix-overflowing-labels-in-the-source-control-sidebar-view"
	demoReleaseBranch  = "release/2026.11"
	demoDevelopBranch  = "develop"
	demoDependaBranch  = "dependabot/go_modules/github.com/some-organisation/some-very-long-module-name-1.2.3"
	demoConflictBranch = "feature/conflicting-edit-of-settings-and-layout"
)

type demoAuthor struct {
	Name  string
	Email string
}

var demoAuthors = []demoAuthor{
	{"Ada Lovelace", "ada@example.invalid"},
	{"Grace Hopper", "grace@example.invalid"},
	{"Linus Torvalds", "linus@example.invalid"},
	{"Margaret Hamilton-Wallace-Montgomery", "margaret@example.invalid"},
	{"Jose Garcia", "jose@example.invalid"},
	{"Sofia Kovalevskaya", "sofia@example.invalid"},
}

var (
	demoTypes  = []string{"feat", "fix", "refactor", "docs", "test", "chore", "perf"}
	demoScopes = []string{"workspace", "gitui", "search", "editor", "palette", "settings", "viewer", "history"}
	demoWords  = []string{"handle", "empty", "state", "when", "loading", "tree", "rows", "overflow", "cache", "results", "focus", "ring", "status", "bar", "tabs"}
)

type demoOp struct {
	Path   string
	Text   string
	Delete bool
}

type historyStream struct {
	buf      bytes.Buffer
	sideMark int
}

func demoWhen(i int) int64 { return demoEpoch + int64(i)*demoCommitStep }

func demoAuthorAt(i int) demoAuthor { return demoAuthors[i%len(demoAuthors)] }

func demoSubject(i int) string {
	subject := fmt.Sprintf("%s(%s): %s %s %s", demoTypes[i%len(demoTypes)], demoScopes[i%len(demoScopes)], demoWords[i%len(demoWords)], demoWords[(i*3+1)%len(demoWords)], demoWords[(i*7+2)%len(demoWords)])
	if i%demoLongSubjectEach == 0 {
		subject += " and keep going with a very long description that never ends so the graph row has to cope with overflow"
	}
	return subject
}

func (s *historyStream) data(text string) {
	fmt.Fprintf(&s.buf, "data %d\n%s\n", len(text), text)
}

func (s *historyStream) commit(ref string, mark int, who demoAuthor, when int64, subject string, from, merge int, ops []demoOp) {
	fmt.Fprintf(&s.buf, "commit %s\nmark :%d\nauthor %s <%s> %d +0000\ncommitter %s <%s> %d +0000\n", ref, mark, who.Name, who.Email, when, who.Name, who.Email, when)
	s.data(subject + "\n")
	if from > 0 {
		fmt.Fprintf(&s.buf, "from :%d\n", from)
	}
	if merge > 0 {
		fmt.Fprintf(&s.buf, "merge :%d\n", merge)
	}
	for _, op := range ops {
		if op.Delete {
			fmt.Fprintf(&s.buf, "D %s\n", op.Path)
			continue
		}
		fmt.Fprintf(&s.buf, "M %s inline %s\n", demoFileMode, op.Path)
		s.data(op.Text)
	}
	s.buf.WriteString("\n")
}

func (s *historyStream) reset(ref string, from int) {
	fmt.Fprintf(&s.buf, "reset %s\nfrom :%d\n\n", ref, from)
}

func demoBaseOps() []demoOp {
	var ops []demoOp
	for _, groups := range [][]demoFile{demoModuleCatalog(), demoLanguageSamples(), demoNestedAndDuplicateFiles(), demoProblemFiles(), demoConflictBase()} {
		for _, f := range groups {
			ops = append(ops, demoOp{Path: f.Path, Text: f.Text})
		}
	}
	ops = append(ops, demoOp{Path: demoBigFile, Text: demoBigFileText()}, demoOp{Path: demoVersionFile, Text: demoVersionText(0)})
	return ops
}

func demoVersionText(i int) string {
	return fmt.Sprintf("package version\n\nconst Version = \"1.0.%d\"\n", i)
}

func demoStream() []byte {
	s := &historyStream{sideMark: demoSideMarkBase}
	const mainRef = "refs/heads/main"
	for i := 1; i <= demoCommitCount; i++ {
		ops := []demoOp{{Path: fmt.Sprintf("%s/entry-%04d.md", demoChangelogDir, i), Text: fmt.Sprintf("# Entry %d\n\n%s\n", i, demoSubject(i))}}
		subject, from, merge := demoSubject(i), 0, 0
		switch {
		case i == 1:
			ops = append(demoBaseOps(), ops...)
			subject = "chore: initial import of the demo project"
		case i == demoConflictMainAt:
			for _, f := range demoConflictMain() {
				ops = append(ops, demoOp{Path: f.Path, Text: f.Text})
			}
			ops = append(ops, demoOp{Path: "conflicts/notes.md", Text: "# Notes\n\nShared line.\n\nEdited on main.\n"})
			subject = "fix(settings): raise the timeout and widen the sidebar"
		case i%demoVersionEvery == 0:
			ops = append(ops, demoOp{Path: demoVersionFile, Text: demoVersionText(i)})
		}
		if i > demoMergeEvery && i%demoMergeEvery == 0 {
			for k := range demoSideCommits {
				s.sideMark++
				prev := 0
				if k == 0 {
					prev = i - 3
				} else {
					prev = s.sideMark - 1
				}
				side := []demoOp{{Path: fmt.Sprintf("topics/topic-%04d-%d.md", i, k), Text: fmt.Sprintf("# Topic %d.%d\n", i, k)}}
				s.commit(fmt.Sprintf("refs/tmp/side%d", i), s.sideMark, demoAuthorAt(i+k+1), demoWhen(i)-int64(demoSideCommits-k)*60, fmt.Sprintf("feat(topic): side work %d part %d", i, k+1), prev, 0, side)
			}
			merge = s.sideMark
			subject = fmt.Sprintf("Merge branch 'topic/%04d'", i)
		}
		s.commit(mainRef, i, demoAuthorAt(i), demoWhen(i), subject, from, merge, ops)
	}
	s.branchWork(demoFeatureBranch, demoCommitCount-40, 4)
	s.branchWork(demoBugfixBranch, demoCommitCount-10, 2)
	s.branchWork(demoDependaBranch, demoCommitCount-20, 1)
	s.sideMark++
	topic := []demoOp{{Path: "conflicts/settings.go", Text: demoConflictTopic()[0].Text}, {Path: "conflicts/layout.json", Text: demoConflictTopic()[1].Text}, {Path: "conflicts/notes.md", Delete: true}}
	s.commit("refs/heads/"+demoConflictBranch, s.sideMark, demoAuthorAt(2), demoWhen(demoConflictBaseAt)+60, "fix(settings): use a shorter timeout and a narrower sidebar", demoConflictBaseAt, 0, topic)
	s.reset("refs/heads/"+demoReleaseBranch, demoCommitCount-300)
	s.reset("refs/heads/"+demoDevelopBranch, demoCommitCount-5)
	return s.buf.Bytes()
}

func (s *historyStream) branchWork(branch string, from, n int) {
	prev := from
	for k := range n {
		s.sideMark++
		path := fmt.Sprintf("branches/%s-%d.md", sanitizeBranch(branch), k)
		s.commit("refs/heads/"+branch, s.sideMark, demoAuthorAt(from+k), demoWhen(from)+int64(k+1)*60, fmt.Sprintf("wip(%s): step %d", demoScopes[k%len(demoScopes)], k+1), prev, 0, []demoOp{{Path: path, Text: fmt.Sprintf("# %s %d\n", branch, k)}})
		prev = 0
	}
}

func sanitizeBranch(branch string) string {
	out := []byte(branch)
	for i, c := range out {
		if c == '/' {
			out[i] = '-'
		}
	}
	return string(out[:min(len(out), 24)])
}
