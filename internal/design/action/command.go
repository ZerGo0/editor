package action

import (
	"sync/atomic"

	"github.com/egoist/mygo/ui"
)

type CommandInfo struct {
	Title    string
	Keys     string
	Disabled bool
}

type Commands interface {
	Lookup(id string) (CommandInfo, bool)
	Run(id string)
}

var commands atomic.Pointer[Commands]

func SetCommands(src Commands) {
	if src == nil {
		commands.Store(nil)
		return
	}
	commands.Store(&src)
}

func Lookup(id string) (CommandInfo, bool) {
	src := commands.Load()
	if src == nil || id == "" {
		return CommandInfo{Title: id, Disabled: true}, false
	}
	info, ok := (*src).Lookup(id)
	if !ok {
		return CommandInfo{Title: id, Disabled: true}, false
	}
	if info.Title == "" {
		info.Title = id
	}
	return info, true
}

func Run(id string) {
	if src := commands.Load(); src != nil {
		(*src).Run(id)
	}
}

func CommandIconButton(c *ui.Context, icon *ui.SVG, id string) ui.Element {
	info, _ := Lookup(id)
	b := IconButton(c, icon, info.Title, IconOptions{Keys: info.Keys, Disabled: info.Disabled, ID: id})
	if b.Clicked() {
		Run(id)
	}
	return b
}

func CommandButton(c *ui.Context, id string, v Variant) ui.Element {
	info, _ := Lookup(id)
	b := Button(c, info.Title, ButtonOptions{Variant: v, Keys: info.Keys, Disabled: info.Disabled, ID: id})
	if b.Clicked() {
		Run(id)
	}
	return b
}

func CommandItem(m *ui.Menu, id string) *ui.MenuItem {
	info, _ := Lookup(id)
	it := Item(m, info.Title, ItemOptions{Detail: info.Keys, Disabled: info.Disabled, ID: id})
	if it.Chosen() {
		Run(id)
	}
	return it
}
