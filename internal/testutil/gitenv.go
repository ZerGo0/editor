package testutil

import (
	"log"
	"os"
)

func IsolateGit(prefix string) (cleanup func(), err error) {
	home, err := os.MkdirTemp("", prefix)
	if err != nil {
		return nil, err
	}
	cleanup = func() {
		if err := os.RemoveAll(home); err != nil {
			log.Printf("remove %s: %v", home, err)
		}
	}
	env := map[string]string{
		"HOME":                home,
		"XDG_CONFIG_HOME":     home,
		"GIT_CONFIG_GLOBAL":   os.DevNull,
		"GIT_CONFIG_NOSYSTEM": "1",
		"GIT_AUTHOR_NAME":     "Ada Lovelace",
		"GIT_AUTHOR_EMAIL":    "ada@example.com",
		"GIT_COMMITTER_NAME":  "Ada Lovelace",
		"GIT_COMMITTER_EMAIL": "ada@example.com",
		"GIT_CONFIG_COUNT":    "3",
		"GIT_CONFIG_KEY_0":    "init.defaultBranch",
		"GIT_CONFIG_VALUE_0":  "main",
		"GIT_CONFIG_KEY_1":    "protocol.file.allow",
		"GIT_CONFIG_VALUE_1":  "always",
		"GIT_CONFIG_KEY_2":    "commit.gpgsign",
		"GIT_CONFIG_VALUE_2":  "false",
	}
	for k, v := range env {
		if err := os.Setenv(k, v); err != nil {
			cleanup()
			return nil, err
		}
	}
	return cleanup, nil
}
