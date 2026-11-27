package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ZerGo0/editor/internal/shellcmd"
)

type TodoAction string

const (
	ActionPick   TodoAction = "pick"
	ActionReword TodoAction = "reword"
	ActionEdit   TodoAction = "edit"
	ActionSquash TodoAction = "squash"
	ActionFixup  TodoAction = "fixup"
	ActionDrop   TodoAction = "drop"
)

const (
	rebaseHelperDir   = "editor-rebase"
	todoFileName      = "todo"
	messageFilePart   = "message-"
	helperDirMode     = 0o700
	helperFileMode    = 0o600
	sequenceEditorEnv = "GIT_SEQUENCE_EDITOR="
	amendCommand      = "exec git commit --amend --allow-empty --no-verify --quiet --file "
	todoFieldCount    = 2
)

type TodoItem struct {
	Action  TodoAction
	OID     string
	Subject string
}

func (a TodoAction) valid() bool {
	switch a {
	case ActionPick, ActionReword, ActionEdit, ActionSquash, ActionFixup, ActionDrop:
		return true
	}
	return false
}

func (a TodoAction) folds() bool { return a == ActionSquash || a == ActionFixup }

func (r *Repo) Rebase(ctx context.Context, onto string) (OpResult, error) {
	if err := checkRef(onto); err != nil {
		return OpResult{}, err
	}
	out, err := r.write(ctx, "rebase", onto)
	return r.finishOp(ctx, out, err)
}

func (r *Repo) RebaseTodo(ctx context.Context, onto string) ([]TodoItem, error) {
	if err := checkRef(onto); err != nil {
		return nil, err
	}
	out, err := r.read(ctx, "log", "-z", "--reverse", "--no-merges", "--right-only", "--cherry-pick",
		"--no-show-signature", "--format=%H%x00%s", onto+"...HEAD", "--")
	if err != nil {
		return nil, err
	}
	fields := splitNUL(out)
	if len(fields)%todoFieldCount != 0 {
		return nil, fmt.Errorf("git log: unexpected field count %d", len(fields))
	}
	items := make([]TodoItem, 0, len(fields)/todoFieldCount)
	for i := 0; i < len(fields); i += todoFieldCount {
		items = append(items, TodoItem{Action: ActionPick, OID: fields[i], Subject: fields[i+1]})
	}
	return items, nil
}

func validateTodo(items []TodoItem) error {
	if len(items) == 0 {
		return errors.New("git rebase: empty todo list")
	}
	kept := 0
	for _, it := range items {
		if !it.Action.valid() {
			return fmt.Errorf("git rebase: unknown action %q", it.Action)
		}
		if err := checkRef(it.OID); err != nil {
			return err
		}
		if it.Action == ActionDrop {
			continue
		}
		if kept == 0 && it.Action.folds() {
			return fmt.Errorf("git rebase: %s needs a previous commit", it.Action)
		}
		kept++
	}
	return nil
}

func nextKeptFolds(items []TodoItem, idx int) bool {
	for _, it := range items[idx+1:] {
		if it.Action != ActionDrop {
			return it.Action.folds()
		}
	}
	return false
}

func (r *Repo) RebaseInteractive(ctx context.Context, onto string, items []TodoItem, messages map[string]string) (OpResult, error) {
	if err := checkRef(onto); err != nil {
		return OpResult{}, err
	}
	if err := validateTodo(items); err != nil {
		return OpResult{}, err
	}
	dir := r.gitPath(rebaseHelperDir)
	r.removeRebaseHelpers()
	if err := os.MkdirAll(dir, helperDirMode); err != nil {
		return OpResult{}, err
	}
	todo, err := buildTodo(dir, items, messages)
	if err != nil {
		r.removeRebaseHelpers()
		return OpResult{}, err
	}
	todoPath := filepath.Join(dir, todoFileName)
	if err := os.WriteFile(todoPath, []byte(todo), helperFileMode); err != nil {
		r.removeRebaseHelpers()
		return OpResult{}, err
	}
	env := []string{sequenceEditorEnv + "cp " + shellcmd.Quote(todoPath)}
	out, err := r.run(ctx, call{env: env}, "rebase", "--interactive", "--no-autosquash", onto)
	res, err := r.finishOp(ctx, out, err)
	if err != nil || !res.Stopped {
		// TECHDEBT: message files live in GitDir until Continue, Skip or Abort sees the rebase end; a rebase finished outside this package leaves them until the next RebaseInteractive; track the rebase in State to clean up sooner.
		r.removeRebaseHelpers()
	}
	return res, err
}

func buildTodo(dir string, items []TodoItem, messages map[string]string) (string, error) {
	var lines []string
	var group []string
	files := 0
	for idx, it := range items {
		action := it.Action
		_, hasMessage := reworded(messages, it.OID)
		if action == ActionReword && hasMessage {
			action = ActionPick
		}
		lines = append(lines, fmt.Sprintf("%s %s %s", action, it.OID, singleLine(it.Subject)))
		if it.Action == ActionDrop {
			continue
		}
		group = append(group, it.OID)
		if nextKeptFolds(items, idx) {
			continue
		}
		for j := len(group) - 1; j >= 0; j-- {
			msg, ok := reworded(messages, group[j])
			if !ok {
				continue
			}
			files++
			path := filepath.Join(dir, messageFilePart+strconv.Itoa(files))
			if err := os.WriteFile(path, []byte(msg), helperFileMode); err != nil {
				return "", err
			}
			lines = append(lines, amendCommand+shellcmd.Quote(path))
			break
		}
		group = nil
	}
	return strings.Join(lines, "\n") + "\n", nil
}

func reworded(messages map[string]string, oid string) (string, bool) {
	msg, ok := messages[oid]
	return msg, ok && strings.TrimSpace(msg) != ""
}

func singleLine(s string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(s, "\n", " ")), " ")
}

func (r *Repo) removeRebaseHelpers() {
	_ = os.RemoveAll(r.gitPath(rebaseHelperDir))
}
