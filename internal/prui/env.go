package prui

import (
	"time"

	"github.com/ZerGo0/editor/internal/ghshared"
	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/github"
	"github.com/egoist/mygo"
)

const (
	confirmButton = 0
	cancelButton  = 1
)

func defaultEnv() Env {
	return Env{
		Token: github.Token,
		NewBackend: func(host, token string) Backend {
			return ghBackend{Client: github.New(host, token), restClient: newRESTClient(host, token)}
		},
		OpenLocal: ghshared.LocalOpener(func(repo *git.Repo) Local { return repo }),
		Checkout:  github.Checkout,
		Confirm:   confirmDialog,
		Now:       time.Now,
		OpenURL:   mygo.Shell.OpenExternal,
		CopyText:  mygo.Clipboard.WriteText,
	}
}

func confirmDialog(win *mygo.Window, opts ConfirmOptions) bool {
	res, err := mygo.Dialog.Message(mygo.MessageOptions{
		Parent:       win,
		Type:         mygo.MessageWarning,
		Message:      opts.Message,
		Detail:       opts.Detail,
		Buttons:      []string{opts.Action, "Cancel"},
		CancelButton: cancelButton,
	})
	return err == nil && res.Button == confirmButton
}
