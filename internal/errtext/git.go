package errtext

import "github.com/ZerGo0/editor/internal/git"

func gitText(err error) (string, bool) {
	return git.Describe(err)
}
