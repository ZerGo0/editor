package gitui

import (
	"errors"

	"github.com/ZerGo0/editor/internal/git"
)

var todoActions = []string{
	string(git.ActionPick),
	string(git.ActionReword),
	string(git.ActionEdit),
	string(git.ActionSquash),
	string(git.ActionFixup),
	string(git.ActionDrop),
}

var errSquashFirst = errors.New("The first kept commit cannot be squashed or fixed up: there is no commit before it to join.")

func moveTodo(items []git.TodoItem, actions []string, i, step int) int {
	j := i + step
	if i < 0 || i >= len(items) || j < 0 || j >= len(items) {
		return i
	}
	items[i], items[j] = items[j], items[i]
	actions[i], actions[j] = actions[j], actions[i]
	return j
}

func buildTodo(items []git.TodoItem, actions []string) ([]git.TodoItem, error) {
	out := make([]git.TodoItem, len(items))
	kept := false
	for i, it := range items {
		it.Action = git.TodoAction(actions[i])
		out[i] = it
		switch it.Action {
		case git.ActionDrop:
		case git.ActionSquash, git.ActionFixup:
			if !kept {
				return nil, errSquashFirst
			}
		default:
			kept = true
		}
	}
	return out, nil
}
