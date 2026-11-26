package layout

const halfPercent = 50

type Node struct {
	Group    *Group
	Vertical bool
	Percent  float32
	First    *Node
	Second   *Node
}

func Leaf(g *Group) *Node {
	return &Node{Group: g}
}

func (n *Node) Groups() []*Group {
	if n.Group != nil {
		return []*Group{n.Group}
	}
	return append(n.First.Groups(), n.Second.Groups()...)
}

func (n *Node) Split(target, added *Group, vertical bool) bool {
	if n.Group != nil {
		if n.Group != target {
			return false
		}
		*n = Node{Vertical: vertical, Percent: halfPercent, First: Leaf(target), Second: Leaf(added)}
		return true
	}
	return n.First.Split(target, added, vertical) || n.Second.Split(target, added, vertical)
}

func (n *Node) Remove(g *Group) bool {
	if n.Group != nil {
		return false
	}
	for _, pair := range [2][2]*Node{{n.First, n.Second}, {n.Second, n.First}} {
		if pair[0].Group == g {
			*n = *pair[1]
			return true
		}
	}
	return n.First.Remove(g) || n.Second.Remove(g)
}
