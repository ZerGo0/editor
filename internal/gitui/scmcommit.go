package gitui

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/design"
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/field"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/ZerGo0/editor/internal/plural"
	"github.com/egoist/mygo/ui"
)

const (
	messageMinLines     = 1
	messageMaxLines     = 10
	messageHistoryLimit = 20
	macCommitKeys       = "\u2318Enter"
	otherCommitKeys     = "Ctrl+Enter"
	darwin              = "darwin"
	commitLabel         = "Commit"
	amendLabel          = "Commit (Amend)"
)

var errAuthor = errors.New("Enter the author as Name <email>.")

type commitMode int

const (
	commitSmart commitMode = iota
	commitStaged
	commitAllFiles
)

type afterCommit int

const (
	afterNone afterCommit = iota
	afterPush
	afterSync
)

type primaryKind int

const (
	primaryCommit primaryKind = iota
	primarySync
	primaryPublish
)

type commitState struct {
	message   string
	amend     bool
	signOff   bool
	noVerify  bool
	author    string
	history   []string
	handle    ui.Handle
	focus     bool
	requested bool

	templateUsed string
}

func commitKeys() string {
	if runtime.GOOS == darwin {
		return macCommitKeys
	}
	return otherCommitKeys
}

func (g *Git) messagePlaceholder() string {
	return fmt.Sprintf("Message (%s to commit on '%s')", commitKeys(), g.headLabel())
}

func (g *Git) commitBox(c *ui.Context) {
	st := g.snap.status
	g.applyTemplate()
	design.ViewBody(c, func() {
		in := field.TextArea(c, "Commit Message", &g.commit.message, field.AreaOptions{
			Placeholder: g.messagePlaceholder(),
			MinLines:    messageMinLines,
			MaxLines:    messageMaxLines,
			ID:          "message",
		})
		if !token.Of(c).IsSkeleton() {
			g.bindMessage(in)
		}
		g.primaryButton(c, st)
		if g.commit.author != "" {
			secondary(c, "Author: "+g.commit.author).SingleLine()
		}
	}).Shrink(0)
	if g.commit.requested {
		g.commit.requested = false
		g.doCommit(commitSmart, afterNone)
	}
}

func (g *Git) bindMessage(in ui.Element) {
	in.Bind(&g.commit.handle).OnTextKey(func(k ui.TextKey) bool {
		if k.Key == ui.KeyEnter && k.Mods == ui.Cmd {
			g.commit.requested = true
			return true
		}
		return false
	})
	if g.commit.focus {
		g.commit.focus = false
		g.commit.handle.Focus()
	}
}

func (g *Git) primaryKind(st *git.Status) primaryKind {
	switch {
	case st == nil || g.commit.amend || strings.TrimSpace(g.commit.message) != "" || g.ChangeCount() > 0:
		return primaryCommit
	case st.Upstream != "" && (st.Ahead > 0 || st.Behind > 0):
		return primarySync
	case st.Upstream == "" && !st.Detached && st.Branch != "" && len(g.snap.remotes) > 0:
		return primaryPublish
	}
	return primaryCommit
}

func (g *Git) primaryButton(c *ui.Context, st *git.Status) {
	busy := g.busy != ""
	o := action.SplitOptions{Variant: action.Primary, Wide: true, ID: "commit", MenuLabel: "Commit Actions", Menu: g.commitMenu}
	switch g.primaryKind(st) {
	case primarySync:
		o.Icon, o.Disabled = icons.Refresh, busy
		label := "Sync Changes " + syncLabel(st.Ahead, st.Behind)
		o.Tooltip = fmt.Sprintf("Pull %d and push %s", st.Behind, plural.Count(st.Ahead, "commit", "commits"))
		if action.SplitButton(c, label, o).Clicked() && !busy {
			g.syncChanges()
		}
	case primaryPublish:
		o.Icon, o.Disabled = icons.Push, busy
		o.Tooltip = "Publish " + st.Branch + " to a remote"
		if action.SplitButton(c, "Publish Branch", o).Clicked() && !busy {
			g.openPush()
		}
	default:
		reason := g.commitBlocker(st, commitSmart)
		label := commitLabel
		if g.commit.amend {
			label = amendLabel
		}
		o.Icon, o.Disabled, o.Tooltip, o.Keys = icons.Check, reason != "", reason, commitKeys()
		if action.SplitButton(c, label, o).Clicked() && reason == "" {
			g.doCommit(commitSmart, afterNone)
		}
	}
}

func (g *Git) commitMenu(m *ui.Menu) {
	st := g.snap.status
	blocked := func(mode commitMode) bool { return g.commitBlocker(st, mode) != "" }
	if m.Item(commitLabel).Disabled(blocked(commitSmart)).Chosen() {
		g.doCommit(commitSmart, afterNone)
	}
	if m.Item("Commit Staged").Disabled(blocked(commitStaged)).Chosen() {
		g.doCommit(commitStaged, afterNone)
	}
	if m.Item("Commit All").Disabled(blocked(commitAllFiles)).Chosen() {
		g.doCommit(commitAllFiles, afterNone)
	}
	m.Separator()
	if m.Item("Commit and Push").Disabled(blocked(commitSmart)).Chosen() {
		g.doCommit(commitSmart, afterPush)
	}
	if m.Item("Commit and Sync").Disabled(blocked(commitSmart) || st == nil || st.Upstream == "").Chosen() {
		g.doCommit(commitSmart, afterSync)
	}
	m.Separator()
	if m.Item("Amend").Checked(g.commit.amend).Chosen() {
		g.toggleAmend()
	}
	m.Submenu("Commit Options", g.commitOptionsMenu)
	m.Submenu("Recent Messages", g.messageHistoryMenu)
}

func (g *Git) commitBlocker(st *git.Status, mode commitMode) string {
	switch {
	case st == nil:
		return "Loading changes."
	case g.busy != "":
		return g.busy + " is running."
	case len(st.Conflicted()) > 0:
		return "Resolve the conflicts before you commit."
	case strings.TrimSpace(g.commit.message) == "" && !g.commit.amend:
		return "Enter a commit message."
	case g.commit.amend:
		return ""
	case mode == commitStaged && len(st.Staged()) == 0:
		return "There are no staged changes to commit."
	case len(st.Staged()) == 0 && len(st.Unstaged()) == 0:
		return "There are no changes to commit."
	}
	return ""
}

func commitPaths(st *git.Status, mode commitMode, amend bool) []string {
	var files []git.FileStatus
	switch {
	case mode == commitStaged:
		return nil
	case mode == commitAllFiles:
		if len(st.Unstaged()) == 0 {
			return nil
		}
		files = append(st.Staged(), st.Unstaged()...)
	case len(st.Staged()) > 0 || amend:
		return nil
	default:
		files = st.Unstaged()
	}
	seen := map[string]bool{}
	var paths []string
	for _, f := range files {
		for _, p := range []string{f.Path, f.OrigPath} {
			if p != "" && !seen[p] {
				seen[p] = true
				paths = append(paths, p)
			}
		}
	}
	return paths
}

func (g *Git) loadAmendMessage() {
	if strings.TrimSpace(g.commit.message) != "" || !g.ready() {
		return
	}
	r := g.repo
	applog.Go(func() {
		msg, err := r.LastCommitMessage(g.ctx)
		g.post(func() {
			if err == nil && g.commit.amend && strings.TrimSpace(g.commit.message) == "" {
				g.commit.message = msg
			}
		})
	})
}

func (g *Git) doCommit(mode commitMode, after afterCommit) {
	st := g.snap.status
	if reason := g.commitBlocker(st, mode); reason != "" {
		if after != afterNone || mode != commitSmart {
			g.h.Notify(reason)
		}
		return
	}
	if !g.h.SaveAllFiles() {
		return
	}
	opts := git.CommitOptions{
		Message:  g.commit.message,
		Amend:    g.commit.amend,
		SignOff:  g.commit.signOff,
		NoVerify: g.commit.noVerify,
		Author:   g.commit.author,
		Paths:    commitPaths(st, mode, g.commit.amend),
	}
	upstream := st.Upstream != ""
	var oid string
	g.do(op{
		title:  "Commit",
		banner: true,
		run: func(ctx context.Context, r repo, _ func(string)) (git.OpResult, error) {
			var err error
			oid, err = r.Commit(ctx, opts)
			return git.OpResult{}, err
		},
		done: func(git.OpResult) {
			g.rememberMessage(opts.Message)
			g.commit.message, g.commit.amend = "", false
			g.h.Notify("Committed " + shortHash(oid))
			g.headChanged()
			switch {
			case after == afterSync:
				g.syncChanges()
			case after == afterPush && upstream:
				g.pushUpstream()
			case after == afterPush:
				g.openPush()
			}
		},
	})
}

func (g *Git) focusCommit() {
	g.commit.focus = true
}

func (g *Git) FocusCommitMessage() bool {
	if !g.ready() || g.snap.status == nil {
		return false
	}
	g.focusCommit()
	return true
}

func (g *Git) conflictCount() int {
	if g.snap.status == nil {
		return 0
	}
	return len(g.snap.status.Conflicted())
}

func (g *Git) commitOptionsMenu(m *ui.Menu) {
	if m.Item("Sign-off Commit").Checked(g.commit.signOff).Chosen() {
		g.commit.signOff = !g.commit.signOff
	}
	if m.Item("Run Git Hooks").Checked(!g.commit.noVerify).Chosen() {
		g.commit.noVerify = !g.commit.noVerify
	}
	if m.Item("Author...").Chosen() {
		g.h.Ask("Commit Author", "Set Author", g.commit.author, func(author string) error {
			author = strings.TrimSpace(author)
			if author != "" && (!strings.Contains(author, "<") || !strings.HasSuffix(author, ">")) {
				return errAuthor
			}
			g.commit.author = author
			return nil
		})
	}
	if m.Item("Reset Author").Disabled(g.commit.author == "").Chosen() {
		g.commit.author = ""
	}
}

func (g *Git) rememberMessage(msg string) {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return
	}
	history := []string{msg}
	for _, m := range g.commit.history {
		if m != msg && len(history) < messageHistoryLimit {
			history = append(history, m)
		}
	}
	g.commit.history = history
}

func (g *Git) messageHistoryMenu(m *ui.Menu) {
	if len(g.commit.history) == 0 {
		m.Item("No Recent Messages").Disabled(true)
		return
	}
	for _, msg := range g.commit.history {
		if m.Item(git.FirstLine(msg)).Chosen() {
			g.commit.message = msg
			g.focusCommit()
		}
	}
}

func opTitle(st git.RepoState) string {
	var b strings.Builder
	switch st.Kind {
	case git.StateMerging:
		b.WriteString("Merging")
	case git.StateRebasing, git.StateRebaseInteractive:
		b.WriteString("Rebasing")
		if st.Head != "" {
			b.WriteString(" " + st.Head)
		}
	case git.StateCherryPicking:
		b.WriteString("Cherry-picking")
	case git.StateReverting:
		b.WriteString("Reverting")
	case git.StateBisecting:
		b.WriteString("Bisecting")
	}
	if st.Head != "" && st.Kind != git.StateRebasing && st.Kind != git.StateRebaseInteractive && st.Kind != git.StateBisecting {
		b.WriteString(" " + shortHash(st.Head))
	}
	if st.Total > 0 {
		fmt.Fprintf(&b, " (%d/%d)", st.Step, st.Total)
	}
	return b.String()
}

func (g *Git) operationBanner(c *ui.Context) {
	st := g.snap.op
	if st.Kind == git.StateNone {
		return
	}
	inset := token.PadView.Px() - token.PadNoticeX.Px()
	ui.Column(c).FillWidth().Shrink(0).Padding(token.GapStack.Px(), inset, token.Space0.Px(), inset).Children(func() {
		g.operationCard(c, st)
	})
}

func (g *Git) operationCard(c *ui.Context, st git.RepoState) {
	p := token.Of(c)
	conflicts := g.conflictCount()
	busy := g.busy != ""
	title := opTitle(st)
	ui.Column(c).FillWidth().Padding(token.PadNoticeY.Px(), token.PadNoticeX.Px()).Gap(token.GapStack.Px()).Radius(token.RadiusCard.Px()).
		Background(p.Severity.WarningBackground).Border(token.BorderWidth.Px(), p.Severity.WarningBorder).
		Role(ui.RoleStatus).Label(title).Children(func() {
		ui.Row(c).FillWidth().Gap(token.GapInline.Px()).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, icons.Warning).Size(token.IconSize.Px(), token.IconSize.Px()).TextColor(p.Severity.Warning).Shrink(0).AccessHidden(true)
			token.TextStrong.Apply(ui.Text(c, title)).TextColor(p.Text.Primary).SingleLine().Grow(1).Shrink(1).MinWidth(0).Tooltip(title).AccessHidden(true)
			action.MenuIconButton(c, icons.More, "Operation Actions", action.IconOptions{}, func(m *ui.Menu) {
				if st.Kind != git.StateBisecting && m.Item("Continue").Disabled(conflicts > 0 || busy).Chosen() {
					g.continueOp()
				}
				if st.Kind != git.StateMerging && st.Kind != git.StateBisecting && m.Item("Skip").Disabled(busy).Chosen() {
					g.skipOp()
				}
				if m.Item("Abort").Danger().Disabled(busy).Chosen() {
					g.abortOp()
				}
			})
		})
		if conflicts > 0 {
			ui.Row(c).FillWidth().Height(token.IconSize.Px()).AlignItems(ui.Center).Children(func() {
				secondary(c, plural.Count(conflicts, "conflicted file", "conflicted files"))
			})
		}
		switch {
		case conflicts > 0:
			if smallButton(c, "Resolve Conflicts", false) {
				g.openConflicts()
			}
		case st.Kind != git.StateBisecting:
			if smallButton(c, "Continue", busy) {
				g.continueOp()
			}
		}
	})
}
