//go:build e2e

package main

import (
	"os"
	"strconv"

	"github.com/ZerGo0/editor/internal/app"
	"github.com/ZerGo0/editor/internal/document"
	"github.com/egoist/mygo"
	_ "github.com/egoist/mygo/plugins/automation"
)

const (
	userDataEnv  = "EDITOR_E2E_USER_DATA"
	editLimitEnv = "EDITOR_E2E_EDIT_LIMIT"
)

func e2eOptions() app.Options {
	dir := os.Getenv(userDataEnv)
	if dir != "" {
		mygo.App.SetPath(mygo.PathUserData, dir)
	}
	if n, err := strconv.ParseInt(os.Getenv(editLimitEnv), 10, 64); err == nil && n > 0 {
		document.EditLimit = n
	}
	mygo.App.SetActivationPolicy(mygo.ActivationPolicyAccessory)
	return app.Options{Hidden: true, DiscardOnClose: true, NoBackups: dir == ""}
}
