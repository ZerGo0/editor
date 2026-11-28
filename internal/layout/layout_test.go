package layout

import (
	"slices"
	"testing"
)

func paths(g *Group) []string {
	var out []string
	for _, t := range g.Tabs {
		out = append(out, t.Path)
	}
	return out
}

func groupWith(active int, names ...string) *Group {
	g := &Group{}
	for i, n := range names {
		g.Tabs = append(g.Tabs, &Tab{ID: i + 1, Path: n})
	}
	g.Active = active
	return g
}

func TestActiveTab(t *testing.T) {
	g := groupWith(0)
	if g.ActiveTab() != nil {
		t.Fatal("empty group should have no active tab")
	}
	g = groupWith(1, "a", "b")
	if got := g.ActiveTab(); got != g.Tabs[1] {
		t.Fatalf("got %v", got)
	}
	g.Active = 5
	if g.ActiveTab() != nil {
		t.Fatal("out of range active should be nil")
	}
	g.Active = -1
	if g.ActiveTab() != nil {
		t.Fatal("negative active should be nil")
	}
}

func TestIndexAndFind(t *testing.T) {
	g := groupWith(0, "a", "b")
	if g.Index(g.Tabs[1]) != 1 || g.Index(&Tab{}) != -1 {
		t.Fatal("Index mismatch")
	}
	if g.Find("b") != 1 || g.Find("z") != -1 {
		t.Fatal("Find mismatch")
	}
}

func TestOpenNewTabInsertsAfterActive(t *testing.T) {
	g := groupWith(0, "a", "b")
	tab := g.Open("c", false, 9)
	if !slices.Equal(paths(g), []string{"a", "c", "b"}) {
		t.Fatalf("paths %v", paths(g))
	}
	if g.Active != 1 || g.ActiveTab() != tab || tab.ID != 9 || tab.Preview {
		t.Fatalf("active %d tab %+v", g.Active, tab)
	}
}

func TestOpenIntoEmptyGroup(t *testing.T) {
	g := &Group{}
	g.Open("a", false, 1)
	if !slices.Equal(paths(g), []string{"a"}) || g.Active != 0 {
		t.Fatalf("paths %v active %d", paths(g), g.Active)
	}
}

func TestOpenExistingActivates(t *testing.T) {
	g := groupWith(2, "a", "b", "c")
	existing := g.Tabs[0]
	got := g.Open("a", false, 99)
	if got != existing || g.Active != 0 || len(g.Tabs) != 3 {
		t.Fatalf("got %+v active %d len %d", got, g.Active, len(g.Tabs))
	}
}

func TestOpenPreview(t *testing.T) {
	g := groupWith(0, "a", "b")
	p := g.Open("p1", true, 10)
	if !p.Preview || !slices.Equal(paths(g), []string{"a", "p1", "b"}) || g.Active != 1 {
		t.Fatalf("paths %v active %d", paths(g), g.Active)
	}
	g.Active = 2
	p2 := g.Open("p2", true, 11)
	if !slices.Equal(paths(g), []string{"a", "p2", "b"}) || g.Active != 1 || g.Tabs[1] != p2 || p2.ID != 11 {
		t.Fatalf("paths %v active %d", paths(g), g.Active)
	}
}

func TestOpenExistingPreviewPinning(t *testing.T) {
	g := groupWith(0, "a")
	p := g.Open("p", true, 2)
	g.Open("p", true, 3)
	if !p.Preview {
		t.Fatal("preview open should keep preview")
	}
	g.Open("p", false, 4)
	if p.Preview {
		t.Fatal("non-preview open should pin")
	}
	if len(g.Tabs) != 2 {
		t.Fatalf("len %d", len(g.Tabs))
	}
}

func TestOpenPreviewKeepsNonPreviewExisting(t *testing.T) {
	g := groupWith(0, "a")
	g.Open("a", true, 5)
	if g.Tabs[0].Preview {
		t.Fatal("existing non-preview tab must stay pinned")
	}
}

func TestClose(t *testing.T) {
	tests := []struct {
		name       string
		active     int
		closeAt    int
		wantPaths  []string
		wantActive int
	}{
		{"before active", 2, 0, []string{"b", "c", "d"}, 1},
		{"at active middle", 1, 1, []string{"a", "c", "d"}, 1},
		{"after active", 0, 2, []string{"a", "b", "d"}, 0},
		{"at active last", 3, 3, []string{"a", "b", "c"}, 2},
		{"at active first", 0, 0, []string{"b", "c", "d"}, 0},
		{"negative index", 1, -1, []string{"a", "b", "c", "d"}, 1},
		{"index too large", 1, 4, []string{"a", "b", "c", "d"}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := groupWith(tt.active, "a", "b", "c", "d")
			g.Close(tt.closeAt)
			if !slices.Equal(paths(g), tt.wantPaths) || g.Active != tt.wantActive {
				t.Fatalf("paths %v active %d", paths(g), g.Active)
			}
		})
	}
}

func TestCloseLastTab(t *testing.T) {
	g := groupWith(0, "a")
	g.Close(0)
	if len(g.Tabs) != 0 || g.Active != 0 || g.ActiveTab() != nil {
		t.Fatalf("tabs %d active %d", len(g.Tabs), g.Active)
	}
	g.Close(0)
}

func TestCycle(t *testing.T) {
	tests := []struct {
		active, delta, want int
	}{
		{0, 1, 1},
		{2, 1, 0},
		{0, -1, 2},
		{1, -1, 0},
		{0, 4, 1},
		{0, -4, 2},
		{1, 0, 1},
	}
	for _, tt := range tests {
		g := groupWith(tt.active, "a", "b", "c")
		g.Cycle(tt.delta)
		if g.Active != tt.want {
			t.Errorf("active %d delta %d: got %d want %d", tt.active, tt.delta, g.Active, tt.want)
		}
	}
	empty := &Group{}
	empty.Cycle(1)
	if empty.Active != 0 {
		t.Fatal("empty cycle changed active")
	}
}

func TestLeafAndGroups(t *testing.T) {
	g := &Group{ID: 1}
	n := Leaf(g)
	if got := n.Groups(); len(got) != 1 || got[0] != g {
		t.Fatalf("got %v", got)
	}
}

func TestSplit(t *testing.T) {
	a, b := &Group{ID: 1}, &Group{ID: 2}
	root := Leaf(a)
	if !root.Split(a, b, true) {
		t.Fatal("split failed")
	}
	if root.Group != nil || !root.Vertical || root.Percent != halfPercent {
		t.Fatalf("root %+v", root)
	}
	if root.First.Group != a || root.Second.Group != b {
		t.Fatal("children wrong")
	}
	if root.Split(&Group{ID: 9}, &Group{ID: 10}, false) {
		t.Fatal("unknown target should fail")
	}
	if got := root.Groups(); !slices.Equal(got, []*Group{a, b}) {
		t.Fatalf("groups %v", got)
	}
}

func TestSplitNested(t *testing.T) {
	a, b, c := &Group{ID: 1}, &Group{ID: 2}, &Group{ID: 3}
	root := Leaf(a)
	root.Split(a, b, false)
	if !root.Split(b, c, true) {
		t.Fatal("nested split failed")
	}
	if root.Vertical || root.Second.Group != nil || !root.Second.Vertical {
		t.Fatalf("tree %+v", root)
	}
	if got := root.Groups(); !slices.Equal(got, []*Group{a, b, c}) {
		t.Fatalf("groups %v", got)
	}
}

func TestRemoveRootLeaf(t *testing.T) {
	a := &Group{ID: 1}
	root := Leaf(a)
	if root.Remove(a) || root.Group != a {
		t.Fatal("root leaf must not be removed")
	}
}

func TestRemoveSiblingTakesParent(t *testing.T) {
	for _, removeFirst := range []bool{true, false} {
		a, b := &Group{ID: 1}, &Group{ID: 2}
		root := Leaf(a)
		root.Split(a, b, true)
		target, keep := b, a
		if removeFirst {
			target, keep = a, b
		}
		if !root.Remove(target) {
			t.Fatal("remove failed")
		}
		if root.Group != keep || root.First != nil || root.Second != nil {
			t.Fatalf("root %+v", root)
		}
	}
}

func TestRemoveNested(t *testing.T) {
	a, b, c := &Group{ID: 1}, &Group{ID: 2}, &Group{ID: 3}
	root := Leaf(a)
	root.Split(a, b, false)
	root.Split(b, c, true)
	if !root.Remove(c) {
		t.Fatal("remove failed")
	}
	if root.Group != nil || root.First.Group != a || root.Second.Group != b {
		t.Fatalf("tree %+v", root)
	}
	if !root.Remove(a) {
		t.Fatal("remove a failed")
	}
	if root.Group != b {
		t.Fatalf("root %+v", root)
	}
	if root.Remove(&Group{ID: 7}) {
		t.Fatal("unknown group removed")
	}
}

func TestRemoveUnknownFromTree(t *testing.T) {
	a, b := &Group{ID: 1}, &Group{ID: 2}
	root := Leaf(a)
	root.Split(a, b, false)
	if root.Remove(&Group{ID: 3}) {
		t.Fatal("unknown group removed")
	}
	if len(root.Groups()) != 2 {
		t.Fatal("tree changed")
	}
}

func TestPinMovesTabToPinnedArea(t *testing.T) {
	g := groupWith(2, "a", "b", "c")
	c := g.Tabs[2]
	c.Preview = true
	g.SetPinned(c, true)
	if got := paths(g); !slices.Equal(got, []string{"c", "a", "b"}) {
		t.Fatalf("after pin: %v", got)
	}
	if !c.Pinned || c.Preview || g.ActiveTab() != c || g.PinnedCount() != 1 {
		t.Fatalf("pinned %v preview %v active %v", c.Pinned, c.Preview, g.ActiveTab())
	}
	b := g.Tabs[2]
	g.SetPinned(b, true)
	if got := paths(g); !slices.Equal(got, []string{"c", "b", "a"}) {
		t.Fatalf("second pin keeps pin order: %v", got)
	}
	g.SetPinned(c, false)
	if got := paths(g); !slices.Equal(got, []string{"b", "c", "a"}) || c.Pinned {
		t.Fatalf("after unpin: %v", got)
	}
}

func TestOpenInsertsAfterPinnedTabs(t *testing.T) {
	g := groupWith(0, "a", "b")
	g.SetPinned(g.Tabs[0], true)
	g.SetPinned(g.Tabs[1], true)
	g.Active = 0
	g.Open("c", true, 9)
	if got := paths(g); !slices.Equal(got, []string{"a", "b", "c"}) {
		t.Fatalf("open after pinned: %v", got)
	}
	g.Open("d", true, 10)
	if got := paths(g); !slices.Equal(got, []string{"a", "b", "d"}) {
		t.Fatalf("preview replaces only unpinned: %v", got)
	}
}

func TestMoveAcrossPinnedBoundary(t *testing.T) {
	g := groupWith(0, "a", "b", "c")
	g.SetPinned(g.Tabs[0], true)
	c := g.Tabs[2]
	g.Move(c, 0)
	if got := paths(g); !slices.Equal(got, []string{"c", "a", "b"}) || !c.Pinned || g.ActiveTab() != c {
		t.Fatalf("drag into pinned area: %v pinned %v", got, c.Pinned)
	}
	g.Move(c, 3)
	if got := paths(g); !slices.Equal(got, []string{"a", "b", "c"}) || c.Pinned {
		t.Fatalf("drag out of pinned area: %v pinned %v", got, c.Pinned)
	}
	other := groupWith(0)
	other.Insert(&Tab{Path: "x"}, 0)
	if other.ActiveTab() == nil || other.Tabs[0].Pinned {
		t.Fatal("insert into an empty group")
	}
}

func TestInsertAfterActive(t *testing.T) {
	g := groupWith(1, "a", "b", "c")
	g.Insert(&Tab{ID: 9, Path: "x"}, AfterActive)
	if !slices.Equal(paths(g), []string{"a", "b", "x", "c"}) || g.ActiveTab().Path != "b" {
		t.Fatalf("paths %v active %d", paths(g), g.Active)
	}
}

func TestInsertAfterActiveStaysBehindPinned(t *testing.T) {
	g := groupWith(0, "a", "b")
	g.SetPinned(g.Tabs[0], true)
	g.SetPinned(g.Tabs[1], true)
	g.Active = 0
	loose := &Tab{ID: 9, Path: "x"}
	g.Insert(loose, AfterActive)
	if loose.Pinned || !slices.Equal(paths(g), []string{"a", "b", "x"}) {
		t.Fatalf("paths %v pinned %v", paths(g), loose.Pinned)
	}
}

func TestInsertAfterActiveKeepsPin(t *testing.T) {
	g := groupWith(2, "a", "b", "c")
	g.SetPinned(g.Tabs[0], true)
	pinned := &Tab{ID: 9, Path: "x", Pinned: true}
	g.Insert(pinned, AfterActive)
	if !pinned.Pinned || !slices.Equal(paths(g), []string{"a", "x", "b", "c"}) || g.ActiveTab().Path != "c" {
		t.Fatalf("paths %v pinned %v active %d", paths(g), pinned.Pinned, g.Active)
	}
}

func TestAddPlacesTheGroupBySide(t *testing.T) {
	for _, tt := range []struct {
		d                  Direction
		vertical, addFirst bool
	}{
		{Left, false, true},
		{Right, false, false},
		{Up, true, true},
		{Down, true, false},
	} {
		a, b := &Group{ID: 1}, &Group{ID: 2}
		root := Leaf(a)
		if !root.Add(a, b, tt.d) {
			t.Fatalf("%v: add failed", tt.d)
		}
		first := root.First.Group
		if root.Vertical != tt.vertical || (first == b) != tt.addFirst {
			t.Fatalf("%v: vertical %v first %v", tt.d, root.Vertical, first.ID)
		}
	}
}

func TestNeighbor(t *testing.T) {
	a, b, c := &Group{ID: 1}, &Group{ID: 2}, &Group{ID: 3}
	root := Leaf(a)
	root.Split(a, b, false)
	root.Split(b, c, true)
	for _, tt := range []struct {
		from *Group
		d    Direction
		want *Group
	}{
		{a, Right, b},
		{b, Left, a},
		{c, Left, a},
		{b, Down, c},
		{c, Up, b},
		{a, Left, nil},
		{a, Up, nil},
		{b, Right, nil},
		{c, Down, nil},
	} {
		if got := root.Neighbor(tt.from, tt.d); got != tt.want {
			t.Errorf("neighbor of %d towards %d: got %v want %v", tt.from.ID, tt.d, got, tt.want)
		}
	}
	if root.Neighbor(&Group{ID: 9}, Left) != nil {
		t.Fatal("unknown group has no neighbor")
	}
}

func TestEvenSharesTheRoomByGroupsAlongEachAxis(t *testing.T) {
	a, b, c := &Group{ID: 1}, &Group{ID: 2}, &Group{ID: 3}
	root := Leaf(a)
	root.Split(a, b, false)
	root.Split(b, c, false)
	root.Percent, root.Second.Percent = 80, 10
	root.Even()
	if got := root.Percent; got < 33 || got > 34 {
		t.Fatalf("first of three groups takes %v%%, want a third", got)
	}
	if root.Second.Percent != 50 {
		t.Fatalf("second split = %v%%, want 50", root.Second.Percent)
	}
}

func TestExpandGivesTheGroupAllTheRoomOnItsPath(t *testing.T) {
	a, b, c := &Group{ID: 1}, &Group{ID: 2}, &Group{ID: 3}
	root := Leaf(a)
	root.Split(a, b, false)
	root.Split(b, c, true)
	root.Expand(c)
	if root.Percent != 0 || root.Second.Percent != 0 {
		t.Fatalf("percents = %v, %v, want 0 and 0", root.Percent, root.Second.Percent)
	}
	if steps := root.Path(a); len(steps) != 1 || !steps[0].First {
		t.Fatalf("path to a = %+v", steps)
	}
}
