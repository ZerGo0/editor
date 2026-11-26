package fuzzy

import (
	"fmt"
	"reflect"
	"testing"
)

func TestFindPositions(t *testing.T) {
	tests := []struct {
		pattern, text string
		want          []int
	}{
		{"ws", "work_space", []int{0, 5}},
		{"abc", "xaxbxc", []int{1, 3, 5}},
		{"view", "preview_list.go", []int{3, 4, 5, 6}},
		{"b", "日本b", []int{2}},
		{"é", "Café", []int{3}},
		{"ü", "Über", []int{0}},
		{"wk vw", "workspace/view.go", []int{0, 3, 10, 13}},
		{"go go", "a.go", []int{2, 3}},
	}
	for _, tt := range tests {
		m, ok := Find(tt.pattern, tt.text)
		if !ok || !reflect.DeepEqual(m.Positions, tt.want) {
			t.Errorf("Find(%q, %q) = %v, %v; want %v", tt.pattern, tt.text, m.Positions, ok, tt.want)
		}
	}
}

func TestFindNoMatch(t *testing.T) {
	for _, tt := range []struct{ pattern, text string }{
		{"xyz", "abc"},
		{"sw", "workspace"},
		{"aa", "a"},
		{"foo", "Fo"},
		{"a zz", "abc"},
		{"x", ""},
	} {
		if _, ok := Find(tt.pattern, tt.text); ok {
			t.Errorf("Find(%q, %q) matched", tt.pattern, tt.text)
		}
	}
}

func TestFindSmartCase(t *testing.T) {
	tests := []struct {
		pattern, text string
		want          bool
	}{
		{"foo", "FOO", true},
		{"foo", "foo", true},
		{"Foo", "foo", false},
		{"Foo", "Foo", true},
		{"fOo", "foo", false},
		{"a BC", "ABC", true},
		{"a Bc", "abc", false},
	}
	for _, tt := range tests {
		if _, ok := Find(tt.pattern, tt.text); ok != tt.want {
			t.Errorf("Find(%q, %q) = %v; want %v", tt.pattern, tt.text, ok, tt.want)
		}
	}
}

func TestFindEmptyPattern(t *testing.T) {
	for _, pattern := range []string{"", "   "} {
		m, ok := Find(pattern, "anything")
		if !ok || m.Score != 0 || len(m.Positions) != 0 {
			t.Errorf("Find(%q) = %+v, %v", pattern, m, ok)
		}
	}
}

func TestFindMultiTermAnyOrder(t *testing.T) {
	a, okA := Find("view work", "workspace/view.go")
	b, okB := Find("work view", "workspace/view.go")
	if !okA || !okB || a.Score != b.Score || !reflect.DeepEqual(a.Positions, b.Positions) {
		t.Errorf("order changed result: %+v %v vs %+v %v", a, okA, b, okB)
	}
}

func TestFindPositionsAscendingUnique(t *testing.T) {
	m, ok := Find("ab ba b", "abab")
	if !ok {
		t.Fatal("no match")
	}
	for i := 1; i < len(m.Positions); i++ {
		if m.Positions[i] <= m.Positions[i-1] {
			t.Fatalf("positions not ascending and unique: %v", m.Positions)
		}
	}
}

func TestFilterRanking(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		items   []string
		want    []string
	}{
		{"boundary beats scattered", "ws", []string{"lowsy", "awxxxxxxxsx", "work_space"}, []string{"work_space", "lowsy", "awxxxxxxxsx"}},
		{"prefix beats inner", "view", []string{"preview_list.go", "view.go"}, []string{"view.go", "preview_list.go"}},
		{"camel hump", "fb", []string{"afxb", "fooBar"}, []string{"fooBar", "afxb"}},
		{"consecutive beats gap", "abc", []string{"xaxbxc", "xabc"}, []string{"xabc", "xaxbxc"}},
		{"last segment", "main", []string{"main/x/util.go", "x/y/main.go"}, []string{"x/y/main.go", "main/x/util.go"}},
		{"shorter wins tie", "a", []string{"aaa", "a", "aa"}, []string{"a", "aa", "aaa"}},
		{"filters non matches", "z", []string{"a", "z", "b"}, []string{"z"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []string
			for _, r := range Filter(tt.pattern, len(tt.items), func(i int) string { return tt.items[i] }) {
				got = append(got, tt.items[r.Index])
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v; want %v", got, tt.want)
			}
		})
	}
}

func TestFilterTieBreaksByIndex(t *testing.T) {
	items := []string{"ab", "ab", "ab"}
	got := Filter("ab", len(items), func(i int) string { return items[i] })
	for i, r := range got {
		if r.Index != i {
			t.Fatalf("got %v", got)
		}
	}
}

func TestFilterEmptyPattern(t *testing.T) {
	items := []string{"c", "a", "b"}
	got := Filter("", len(items), func(i int) string { return items[i] })
	if len(got) != len(items) {
		t.Fatalf("got %d results", len(got))
	}
	for i, r := range got {
		if r.Index != i || r.Match.Score != 0 || len(r.Match.Positions) != 0 {
			t.Errorf("result %d = %+v", i, r)
		}
	}
}

func TestFilterZeroItems(t *testing.T) {
	if got := Filter("a", 0, nil); len(got) != 0 {
		t.Errorf("got %v", got)
	}
}

func BenchmarkFilter(b *testing.B) {
	const count = 100000
	paths := make([]string, count)
	for i := range paths {
		paths[i] = fmt.Sprintf("internal/module%d/pkg%d/file_%d_handler.go", i%97, i%31, i)
	}
	text := func(i int) string { return paths[i] }
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Filter("mod hand", count, text)
	}
}
