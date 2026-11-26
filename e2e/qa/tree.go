package qa

import "slices"

type Node struct {
	ID           string   `json:"id"`
	Parent       string   `json:"parent"`
	Role         string   `json:"role"`
	Key          any      `json:"key"`
	Label        string   `json:"label"`
	Value        string   `json:"value"`
	Placeholder  string   `json:"placeholder"`
	Description  string   `json:"description"`
	KeyShortcuts string   `json:"keyShortcuts"`
	States       []string `json:"states"`
	Actions      []string `json:"actions"`
	Bounds       Rect     `json:"bounds"`
}

func (n Node) Has(state string) bool { return slices.Contains(n.States, state) }

func (n Node) Name() string {
	switch {
	case n.Label != "":
		return n.Label
	case n.Value != "":
		return n.Value
	}
	return n.Placeholder
}

type Tree struct {
	Focus  string `json:"focus"`
	Nodes  []Node `json:"nodes"`
	Window Rect   `json:"-"`
	byID   map[string]int
}

func (t *Tree) Index() {
	t.byID = make(map[string]int, len(t.Nodes))
	for i, n := range t.Nodes {
		t.byID[n.ID] = i
	}
}

func (t *Tree) ByID(id string) (Node, bool) {
	if t.byID == nil {
		t.Index()
	}
	i, ok := t.byID[id]
	if !ok {
		return Node{}, false
	}
	return t.Nodes[i], true
}

func (t *Tree) Ancestors(n Node) []Node {
	var out []Node
	seen := map[string]bool{n.ID: true}
	for p, ok := t.ByID(n.Parent); ok && !seen[p.ID]; p, ok = t.ByID(p.Parent) {
		seen[p.ID] = true
		out = append(out, p)
	}
	return out
}

func (t *Tree) Focused() (Node, bool) {
	if t.Focus == "" {
		return Node{}, false
	}
	return t.ByID(t.Focus)
}

func (t *Tree) Find(match func(Node) bool) (Node, bool) {
	for _, n := range t.Nodes {
		if match(n) {
			return n, true
		}
	}
	return Node{}, false
}

var controlRoles = map[string]bool{
	"button": true, "link": true, "checkBox": true, "radio": true, "switch": true, "slider": true,
	"textField": true, "popUpButton": true, "menuButton": true, "toggleButton": true, "comboBox": true,
	"stepper": true, "tab": true, "treeItem": true, "listItem": true, "menuItem": true,
	"menuItemCheckBox": true, "menuItemRadio": true, "disclosure": true, "colorWell": true,
}

func ControlRole(role string) bool { return controlRoles[role] }
