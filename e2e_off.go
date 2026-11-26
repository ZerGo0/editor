//go:build !e2e

package main

import "github.com/ZerGo0/editor/internal/app"

func e2eOptions() app.Options {
	return app.Options{}
}
