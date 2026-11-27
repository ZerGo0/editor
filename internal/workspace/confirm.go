package workspace

import (
	"fmt"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/egoist/mygo"
)

type saveChoice int

const (
	choiceSave saveChoice = iota
	choiceDiscard
	choiceCancel
)

func askSave(win *mygo.Window, names []string) saveChoice {
	message := fmt.Sprintf("Do you want to save the changes to %s?", names[0])
	if len(names) > 1 {
		message = fmt.Sprintf("Do you want to save the changes to %d files?", len(names))
	}
	res, err := mygo.Dialog.Message(mygo.MessageOptions{
		Parent:             win,
		Type:               mygo.MessageWarning,
		Message:            message,
		Detail:             "If you do not save them, your changes are lost.",
		Buttons:            []string{"Save", "Don't Save", "Cancel"},
		CancelButton:       int(choiceCancel),
		DestructiveButtons: []int{int(choiceDiscard)},
	})
	if err != nil || res.Button < int(choiceSave) || res.Button > int(choiceCancel) {
		return choiceCancel
	}
	return saveChoice(res.Button)
}

func (w *Workspace) ConfirmClose(reason CloseReason, done func(ok bool)) {
	dirty := w.unsaved(reason)
	if len(dirty) == 0 || w.win == nil {
		done(true)
		return
	}
	names := make([]string, len(dirty))
	for i, d := range dirty {
		names[i] = w.tabTitle(d.Path)
	}
	applog.Go(func() {
		choice := askSave(w.win, names)
		w.update(func() {
			switch choice {
			case choiceSave:
				w.saveThen(dirty, func() {
					w.discardBackups(dirty)
					done(true)
				})
			case choiceDiscard:
				w.discardBackups(dirty)
				done(true)
			default:
				done(false)
			}
		})
	})
}
