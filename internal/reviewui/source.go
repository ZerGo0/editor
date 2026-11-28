package reviewui

import (
	"context"

	"github.com/ZerGo0/editor/internal/multidiff"
	"github.com/ZerGo0/editor/internal/review"
	"github.com/egoist/mygo/ui/editor"
)

type Decorate = func([]multidiff.Entry) []multidiff.Entry

type Source interface {
	View() *multidiff.View
	Options() multidiff.Options
	Status() (loaded, loading bool, err error)
	Reload()
	Close()
}

type Changes interface {
	Source
	Base() string
}

type PullRequest interface {
	Source
	PR() review.PR
}

type Deps struct {
	Ctx       context.Context
	Post      func(func())
	Decide    func(review.Result) error
	Changes   func(mode review.DiffMode, base string, decorate Decorate) Changes
	PR        func(ref review.PRRef, decorate Decorate) PullRequest
	Configure func(d *editor.DiffState, path string)
	Language  func(path, text string) *editor.Language
	ReadFile  func(path string) (string, error)
}
