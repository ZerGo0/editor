package gitui

import (
	"context"
	"math"
	"slices"
	"strconv"

	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/git"
)

const (
	cancelCommand = "git.cancel"
	cancelPrefix  = "Cancel "
	percentScale  = 100
	ellipsis      = "..."
)

type task struct {
	title      string
	phase      string
	line       string
	value      float64
	cancel     context.CancelFunc
	cancelable bool
	blocking   bool
	detached   bool
}

func (g *Git) addTask(title string, cancel context.CancelFunc, cancelable, blocking bool) *task {
	t := &task{title: title, value: feedback.Indeterminate, cancel: cancel, cancelable: cancelable, blocking: blocking}
	g.tasks = append(g.tasks, t)
	if blocking {
		g.busy = title
	}
	return t
}

func (g *Git) removeTask(t *task) {
	if t == nil {
		return
	}
	g.tasks = slices.DeleteFunc(g.tasks, func(o *task) bool { return o == t })
	if t.blocking {
		g.busy = ""
	}
}

func (g *Git) currentTask() *task {
	if len(g.tasks) == 0 {
		return nil
	}
	return g.tasks[len(g.tasks)-1]
}

func (g *Git) remoteTask() *task {
	for _, t := range g.tasks {
		if t.cancelable && !t.detached {
			return t
		}
	}
	return nil
}

func (g *Git) taskNamed(title string) *task {
	for _, t := range g.tasks {
		if t.title == title {
			return t
		}
	}
	return nil
}

func (g *Git) cancelTask() {
	for i := len(g.tasks) - 1; i >= 0; i-- {
		if t := g.tasks[i]; t.cancelable {
			t.cancel()
			return
		}
	}
	g.h.Notify("No Git operation that can be cancelled is running.")
}

func (t *task) report(line string) {
	if t == nil {
		return
	}
	t.line = line
	if phase, done, ok := git.ParseProgress(line); ok {
		t.phase, t.value = phase, done
	}
}

func (t *task) label() string {
	switch {
	case t.phase != "":
		return t.title + ": " + t.phase
	case t.line != "":
		return t.title + ": " + t.line
	}
	return t.title + ellipsis
}

func (t *task) statusText() string {
	if t.value < 0 {
		return t.title + ellipsis
	}
	return t.title + " " + strconv.Itoa(int(math.Round(t.value*percentScale))) + "%"
}

func (t *task) cancelLabel() string {
	return cancelPrefix + t.title
}
