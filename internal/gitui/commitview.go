package gitui

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

const (
	messageMinLines     = 3
	messageMaxLines     = 10
	bannerRadius        = 6
	messageHistoryLimit = 20
)

var errAuthor = errors.New("Enter the author as Name <email>.")

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

func (g *Git) SideView(c *ui.Context) {
	ui.Column(c).Fill().Children(func() {
		if g.gate(c) {
			g.commitWindow(c)
		}
	})
}

func (g *Git) gate(c *ui.Context) bool {
	switch g.phase {
	case phaseLoading:
		loadingState(c, "Loading repository")
	case phaseNoGit:
		emptyState(c, "Git is not installed", "Install Git, then reopen this folder.")
	case phaseNoRepo:
		g.noRepoView(c)
	case phaseError:
		errorState(c, "Cannot open the repository", g.openErr, g.open)
	default:
		return g.repo != nil
	}
	return false
}

func (g *Git) noRepoView(c *ui.Context) {
	ui.Column(c).FillWidth().Grow(1).Center().Gap(stateGap).Padding(statePadding).Children(func() {
		ui.Text(c, "This folder is not a Git repository").FontSize(bodyFont).TextColor(c.Theme().TextMuted).TextAlign(ui.Center)
		if ui.PrimaryButton(c, "Initialize Repository").Clicked() {
			g.initRepository()
		}
	})
}

func (g *Git) commitWindow(c *ui.Context) {
	g.repoHeader(c)
	g.busyLine(c)
	g.operationBanner(c)
	switch {
	case g.snap.status == nil && g.snap.err != nil:
		errorState(c, "Cannot read the status", g.snap.err, g.refreshNow)
		return
	case g.snap.status == nil:
		loadingState(c, "Loading changes")
		return
	}
	if g.snap.err != nil {
		ui.Text(c, g.snap.err.Error()).FontSize(smallFont).TextColor(c.Theme().Danger).Padding(0, rowPadding)
	}
	g.changesList(c)
	g.commitPanel(c)
}

func (g *Git) repoHeader(c *ui.Context) {
	t := c.Theme()
	busy := g.busy != ""
	ui.Row(c).FillWidth().Height(toolbarHeight).Shrink(0).Padding(0, rowPadding).Gap(2).AlignItems(ui.Center).Children(func() {
		name := g.headLabel()
		b := ui.ButtonBase(c.Key("branch")).Gap(4).Padding(2, 6).Radius(buttonRadius).AlignItems(ui.Center).
			Label("Branch " + name).Tooltip("Branches").Shrink(1).MinWidth(0)
		if b.Hovered() {
			b.Background(t.SurfaceHover)
		}
		b.Children(func() {
			ui.Icon(c, icons.GitBranch).Size(iconSize, iconSize)
			ui.Text(c, name).FontSize(bodyFont).Bold().SingleLine().Shrink(1).MinWidth(0)
			if st := g.snap.status; st != nil {
				if s := syncLabel(st.Ahead, st.Behind); s != "" {
					muted(c, s).SingleLine().Shrink(0)
				}
			}
		})
		b.OnClick(g.openBranches)
		ui.Box(c).Grow(1)
		if iconButton(c, "Refresh", icons.Refresh, false).Clicked() {
			g.refreshNow()
		}
		if textButton(c, "Fetch", busy).Clicked() {
			g.fetch()
		}
		if iconButton(c, "Pull", icons.Pull, busy).Clicked() {
			g.pull()
		}
		if iconButton(c, "Push", icons.Push, busy).Clicked() {
			g.openPush()
		}
		iconButton(c, "More Git Actions", icons.More, false).PopoverMenuButton(g.moreMenu)
	})
}

func (g *Git) moreMenu(m *ui.Menu) {
	if m.Item("New Branch...").Chosen() {
		g.newBranch("")
	}
	if m.Item("Show Log").Chosen() {
		g.showLog()
	}
	m.Separator()
	if m.Item("Stash Changes...").Chosen() {
		g.openStash()
	}
	if m.Item("Stashes...").Chosen() {
		g.openStashes()
	}
	if m.Item("Shelf...").Chosen() {
		g.openShelf()
	}
	m.Separator()
	if m.Item("Resolve Conflicts...").Disabled(g.conflictCount() == 0).Chosen() {
		g.openConflicts()
	}
	if m.Item("Worktrees...").Chosen() {
		g.openWorktrees()
	}
	if m.Item("Submodules...").Chosen() {
		g.openSubmodules()
	}
}

func (g *Git) busyLine(c *ui.Context) {
	if g.busy == "" {
		return
	}
	ui.Row(c).FillWidth().Padding(0, rowPadding, 4, rowPadding).Gap(6).AlignItems(ui.Center).Children(func() {
		ui.Spinner(c).Size(iconSize, iconSize).Label(g.busy)
		ui.Text(c, g.busy+"...").FontSize(smallFont).SingleLine().Shrink(0)
		if g.progress != "" {
			muted(c, g.progress).SingleLine().Shrink(1).MinWidth(0)
		}
	})
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
	t := c.Theme()
	conflicts := g.conflictCount()
	busy := g.busy != ""
	ui.Column(c).FillWidth().Margin(0, rowPadding, 6, rowPadding).Padding(6, rowPadding).Gap(6).Radius(bannerRadius).
		Background(t.Warning.Alpha(tintAlpha)).Role(ui.RoleStatus).Label(opTitle(st)).Children(func() {
		ui.Text(c, opTitle(st)).FontSize(bodyFont).Bold()
		if conflicts > 0 {
			muted(c, fmt.Sprintf("%d conflicted files", conflicts))
		}
		ui.Row(c).Gap(4).Wrap().Children(func() {
			if conflicts > 0 && ui.Button(c, "Resolve Conflicts").Clicked() {
				g.openConflicts()
			}
			if st.Kind != git.StateBisecting && ui.Button(c, "Continue").Disabled(conflicts > 0 || busy).Clicked() {
				g.continueOp()
			}
			if st.Kind != git.StateMerging && st.Kind != git.StateBisecting && ui.Button(c, "Skip").Disabled(busy).Clicked() {
				g.skipOp()
			}
			if ui.Button(c, "Abort").Disabled(busy).Clicked() {
				g.abortOp()
			}
		})
	})
}

func (g *Git) commitPanel(c *ui.Context) {
	t := c.Theme()
	st := g.snap.status
	g.applyTemplate()
	ui.Column(c).FillWidth().Shrink(0).Padding(rowPadding).Gap(6).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Children(func() {
		ui.TextArea(c.Key("message"), &g.commit.message).Lines(messageMinLines, messageMaxLines).FillWidth().
			Placeholder("Commit Message").Label("Commit Message").Bind(&g.commit.handle).
			OnTextKey(func(k ui.TextKey) bool {
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
		reason := g.commitBlocker(st)
		ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
			if ui.Checkbox(c, &g.commit.amend, "Amend").Changed() && g.commit.amend {
				g.loadAmendMessage()
			}
			ui.Box(c).Grow(1)
			if g.commit.author != "" {
				muted(c, "Author: "+g.commit.author).SingleLine().Shrink(1).MinWidth(0)
			}
			iconButton(c, "Message History", icons.History, len(g.commit.history) == 0).PopoverMenuButton(g.messageHistoryMenu)
			iconButton(c, "Commit Options", icons.More, false).PopoverMenuButton(g.commitOptionsMenu)
		})
		if reason != "" {
			muted(c, reason)
		} else if st != nil && len(st.Staged()) == 0 && !g.commit.amend {
			muted(c, "Nothing is staged: all changed tracked files will be committed.")
		}
		ui.Row(c).FillWidth().Gap(6).Justify(ui.End).Children(func() {
			if ui.Button(c, "Commit and Push").Disabled(reason != "").Clicked() {
				g.doCommit(true)
			}
			if ui.PrimaryButton(c, "Commit").Disabled(reason != "").Clicked() {
				g.doCommit(false)
			}
		})
	})
	if g.commit.requested {
		g.commit.requested = false
		if g.commitBlocker(st) == "" {
			g.doCommit(false)
		}
	}
}

func (g *Git) commitBlocker(st *git.Status) string {
	switch {
	case st == nil:
		return "Loading changes."
	case g.busy != "":
		return g.busy + " is running."
	case len(st.Conflicted()) > 0:
		return "Resolve the conflicts before you commit."
	case strings.TrimSpace(g.commit.message) == "" && !g.commit.amend:
		return "Enter a commit message."
	case !g.commit.amend && len(st.Staged()) == 0 && len(st.Unstaged()) == 0:
		return "There are no changes to commit."
	}
	return ""
}

func (g *Git) loadAmendMessage() {
	if strings.TrimSpace(g.commit.message) != "" || !g.ready() {
		return
	}
	r := g.repo
	go func() {
		msg, err := r.LastCommitMessage(g.ctx)
		g.post(func() {
			if err == nil && g.commit.amend && strings.TrimSpace(g.commit.message) == "" {
				g.commit.message = msg
			}
		})
	}()
}

func (g *Git) doCommit(push bool) {
	st := g.snap.status
	if g.commitBlocker(st) != "" {
		return
	}
	var paths []string
	if len(st.Staged()) == 0 && !g.commit.amend {
		for _, f := range st.Unstaged() {
			paths = append(paths, f.Path)
			if f.OrigPath != "" {
				paths = append(paths, f.OrigPath)
			}
		}
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
		Paths:    paths,
	}
	var oid string
	g.do(op{
		title: "Commit",
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
			if push {
				g.openPush()
			}
		},
	})
}

func (g *Git) focusCommit() {
	g.commit.focus = true
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
	for _, msg := range g.commit.history {
		if m.Item(firstLineOf(msg)).Chosen() {
			g.commit.message = msg
			g.focusCommit()
		}
	}
}
