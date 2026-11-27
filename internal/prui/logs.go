package prui

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/design/detail"
	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

var ansiPattern = regexp.MustCompile("\x1b\\[[0-9;?]*[ -/]*[@-~]")

type logTab struct {
	name    string
	jobID   int64
	loading bool
	err     error
	state   *editor.State
	gen     int
}

func stripANSI(s string) string {
	return ansiPattern.ReplaceAllString(s, "")
}

func (p *PRs) openLogs(d *prData, chk checkRow) {
	id := fmt.Sprintf("%s/log/%d", detailID(p.repo, d.number), chk.JobID)
	t := p.logs[id]
	if t == nil {
		t = &logTab{name: chk.Name, jobID: chk.JobID}
		p.logs[id] = t
	}
	if t.state == nil && !t.loading {
		p.loadLogs(t)
	}
	p.h.OpenView(id, "Logs: "+chk.Name, func(c *ui.Context) { p.logView(c, t) })
}

func (p *PRs) loadLogs(t *logTab) {
	t.gen++
	gen := t.gen
	t.loading, t.err = true, nil
	be, repo, job := p.be, p.repo, t.jobID
	applog.Go(func() {
		text, err := be.JobLogs(p.ctx, repo, job)
		p.post(func() {
			if gen != t.gen {
				return
			}
			t.loading = false
			if err != nil {
				t.err = err
				return
			}
			buf := editor.NewBuffer(strings.TrimRight(stripANSI(text), "\r\n"))
			t.state = &editor.State{Buffer: buf, ReadOnly: true}
		})
	})
}

func (p *PRs) logView(c *ui.Context, t *logTab) {
	if p.closed {
		return
	}
	detail.Page(c, func() {
		switch {
		case t.err != nil:
			detail.Centered(c, func() {
				feedback.Error(c, "Cannot load the logs of "+t.name, t.err, func() { p.loadLogs(t) })
			})
		case t.state == nil:
			detail.Centered(c, func() { feedback.Loading(c, "Loading logs...") })
		default:
			p.h.ConfigureEditor(t.state, "")
			editor.View(c, t.state).Grow(1).FillWidth().Label("Logs of " + t.name)
		}
	})
}

func (p *PRs) runAction(d *prData, chk checkRow, action string) {
	repo, run := p.repo, chk.RunID
	p.act(&d.busy, "Updating the workflow run", "Cannot update the workflow run", func(ctx context.Context) (func(), error) {
		if err := p.be.RunAction(ctx, repo, run, action); err != nil {
			return nil, err
		}
		return func() {
			p.h.Notify("Updated the workflow run of " + chk.Name)
			p.loadDetail(d)
		}, nil
	})
}
