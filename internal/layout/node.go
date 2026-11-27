package layout

const (
	halfPercent  = 50
	fullPercent  = 100
	touchEpsilon = 0.001
)

type Direction int

const (
	Left Direction = iota
	Right
	Up
	Down
)

func (d Direction) Vertical() bool { return d == Up || d == Down }

func (d Direction) Before() bool { return d == Left || d == Up }

type area struct{ x, y, w, h float32 }

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
	return n.split(target, added, vertical, false)
}

func (n *Node) Add(target, added *Group, d Direction) bool {
	return n.split(target, added, d.Vertical(), d.Before())
}

func (n *Node) split(target, added *Group, vertical, before bool) bool {
	if n.Group != nil {
		if n.Group != target {
			return false
		}
		first, second := Leaf(target), Leaf(added)
		if before {
			first, second = second, first
		}
		*n = Node{Vertical: vertical, Percent: halfPercent, First: first, Second: second}
		return true
	}
	return n.First.split(target, added, vertical, before) || n.Second.split(target, added, vertical, before)
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

func (n *Node) areas(a area, out map[*Group]area) {
	if n.Group != nil {
		out[n.Group] = a
		return
	}
	share := n.Percent / fullPercent
	if n.Vertical {
		n.First.areas(area{a.x, a.y, a.w, a.h * share}, out)
		n.Second.areas(area{a.x, a.y + a.h*share, a.w, a.h * (1 - share)}, out)
		return
	}
	n.First.areas(area{a.x, a.y, a.w * share, a.h}, out)
	n.Second.areas(area{a.x + a.w*share, a.y, a.w * (1 - share), a.h}, out)
}

func (n *Node) Neighbor(g *Group, d Direction) *Group {
	areas := map[*Group]area{}
	n.areas(area{0, 0, 1, 1}, areas)
	from, ok := areas[g]
	if !ok {
		return nil
	}
	var best *Group
	var bestShare float32
	for _, c := range n.Groups() {
		to := areas[c]
		if c == g || !adjacent(from, to, d) {
			continue
		}
		if share := sharedEdge(from, to, d); share > touchEpsilon && share > bestShare+touchEpsilon {
			best, bestShare = c, share
		}
	}
	return best
}

func adjacent(from, to area, d Direction) bool {
	switch d {
	case Left:
		return near(to.x+to.w, from.x)
	case Right:
		return near(to.x, from.x+from.w)
	case Up:
		return near(to.y+to.h, from.y)
	}
	return near(to.y, from.y+from.h)
}

func sharedEdge(from, to area, d Direction) float32 {
	if d.Vertical() {
		return min(from.x+from.w, to.x+to.w) - max(from.x, to.x)
	}
	return min(from.y+from.h, to.y+to.h) - max(from.y, to.y)
}

func near(a, b float32) bool {
	diff := a - b
	return diff < touchEpsilon && diff > -touchEpsilon
}
