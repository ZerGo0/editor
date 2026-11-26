package command

import (
	"cmp"
	"slices"

	"github.com/egoist/mygo"
)

type Command struct {
	ID       string
	Title    string
	Category string
	Run      func(win *mygo.Window)
}

func (c *Command) Label() string {
	if c.Category == "" {
		return c.Title
	}
	return c.Category + ": " + c.Title
}

type Registry struct {
	byID   map[string]*Command
	all    []*Command
	inMenu map[string]bool
	recent []string
}

const maxRecent = 20

func NewRegistry() *Registry {
	return &Registry{byID: map[string]*Command{}, inMenu: map[string]bool{}}
}

func (r *Registry) Add(cmds ...*Command) {
	for _, c := range cmds {
		if _, dup := r.byID[c.ID]; dup {
			panic("command: duplicate id " + c.ID)
		}
		r.byID[c.ID] = c
		r.all = append(r.all, c)
	}
	slices.SortFunc(r.all, func(a, b *Command) int { return cmp.Compare(a.Label(), b.Label()) })
}

func (r *Registry) Get(id string) *Command {
	return r.byID[id]
}

func (r *Registry) All() []*Command {
	return r.all
}

func (r *Registry) Run(id string, win *mygo.Window) bool {
	c := r.byID[id]
	if c == nil {
		return false
	}
	r.recent = slices.Insert(slices.DeleteFunc(r.recent, func(s string) bool { return s == id }), 0, id)
	if len(r.recent) > maxRecent {
		r.recent = r.recent[:maxRecent]
	}
	c.Run(win)
	return true
}

func (r *Registry) Recent() []string {
	return r.recent
}

func (r *Registry) SetInMenu(id string) {
	r.inMenu[id] = true
}

func (r *Registry) InMenu(id string) bool {
	return r.inMenu[id]
}
