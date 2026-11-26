package git

import (
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func render(chunks []Chunk) string {
	var parts []string
	for _, c := range chunks {
		switch c.Kind {
		case ChunkStable:
			parts = append(parts, "S("+strings.Join(c.Ours, " ")+")")
		case ChunkOurs:
			parts = append(parts, "O("+strings.Join(c.Ours, " ")+")")
		case ChunkTheirs:
			parts = append(parts, "T("+strings.Join(c.Theirs, " ")+")")
		case ChunkConflict:
			parts = append(parts, "C("+strings.Join(c.Ours, " ")+"|"+strings.Join(c.Theirs, " ")+")")
		}
	}
	return strings.Join(parts, " ")
}

func lines(s string) []string { return strings.Fields(s) }

func TestMerge3Table(t *testing.T) {
	cases := []struct {
		name, base, ours, theirs, want string
	}{
		{"identical", "a b c", "a b c", "a b c", "S(a b c)"},
		{"ours only", "a b c d e", "a B c d e", "a b c d e", "S(a) O(B) S(c d e)"},
		{"theirs only", "a b c d e", "a b c d e", "a b c D e", "S(a b c) T(D) S(e)"},
		{"both sides apart", "a b c d e", "a B c d e", "a b c D e", "S(a) O(B) S(c) T(D) S(e)"},
		{"same change", "a b c", "a X c", "a X c", "S(a) S(X) S(c)"},
		{"conflict", "a b c", "a X c", "a Y c", "S(a) C(X|Y) S(c)"},
		{"adjacent edits conflict", "a b c d", "a B c d", "a b C d", "S(a) C(B c|b C) S(d)"},
		{"ours inserts", "a b", "a n b", "a b", "S(a) O(n) S(b)"},
		{"theirs deletes", "a b c", "a b c", "a c", "S(a) T() S(c)"},
		{"ours deletes theirs edits same line", "a b c", "a c", "a B c", "S(a) C(|B) S(c)"},
		{"both delete", "a b c", "a c", "a c", "S(a) S() S(c)"},
		{"both insert different", "a c", "a x c", "a y c", "S(a) C(x|y) S(c)"},
		{"both insert same", "a c", "a x c", "a x c", "S(a) S(x) S(c)"},
		{"empty base", "", "x", "y", "C(x|y)"},
		{"empty base same", "", "x", "x", "S(x)"},
		{"all empty", "", "", "", ""},
		{"ours appends theirs prepends", "m", "m z", "a m", "T(a) S(m) O(z)"},
		{"whole file replaced by theirs", "a b", "a b", "x y z", "T(x y z)"},
		{"repeated lines", "a a a", "a a a b", "c a a a", "T(c) S(a a a) O(b)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := render(Merge3(lines(c.base), lines(c.ours), lines(c.theirs)))
			if got != c.want {
				t.Fatalf("got  %s\nwant %s", got, c.want)
			}
		})
	}
}

func TestMerge3KeepsBaseText(t *testing.T) {
	chunks := Merge3(lines("a b c"), lines("a X c"), lines("a Y c"))
	conflict := chunks[1]
	if conflict.Kind != ChunkConflict || strings.Join(conflict.Base, "") != "b" {
		t.Fatalf("conflict chunk: %+v", conflict)
	}
}

func lcsLength(a, b []int) int {
	prev := make([]int, len(b)+1)
	for i := range a {
		cur := make([]int, len(b)+1)
		for j := range b {
			switch {
			case a[i] == b[j]:
				cur[j+1] = prev[j] + 1
			case prev[j+1] >= cur[j]:
				cur[j+1] = prev[j+1]
			default:
				cur[j+1] = cur[j]
			}
		}
		prev = cur
	}
	return prev[len(b)]
}

func TestMatchLinesIsOptimalLCS(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	for trial := 0; trial < 600; trial++ {
		alphabet := 2 + rng.Intn(6)
		a := make([]int, rng.Intn(40))
		b := make([]int, rng.Intn(40))
		for i := range a {
			a[i] = rng.Intn(alphabet)
		}
		for i := range b {
			b[i] = rng.Intn(alphabet)
		}
		pairs := matchLines(a, b)
		prevA, prevB := -1, -1
		for _, p := range pairs {
			if p.a <= prevA || p.b <= prevB || a[p.a] != b[p.b] {
				t.Fatalf("invalid pairs %v for %v %v", pairs, a, b)
			}
			prevA, prevB = p.a, p.b
		}
		if want := lcsLength(a, b); len(pairs) != want {
			t.Fatalf("trial %d: matched %d want %d\na=%v\nb=%v", trial, len(pairs), want, a, b)
		}
	}
}

func bigFile(rng *rand.Rand, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = "line " + itoa(rng.Intn(n*2))
	}
	return out
}

func TestMerge3Performance(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	base := bigFile(rng, 4000)
	ours := append([]string(nil), base...)
	theirs := append([]string(nil), base...)
	for i := 0; i < 200; i++ {
		ours[rng.Intn(len(ours))] = "ours " + itoa(i)
		theirs[rng.Intn(len(theirs))] = "theirs " + itoa(i)
	}
	start := time.Now()
	chunks := Merge3(base, ours, theirs)
	if len(chunks) == 0 {
		t.Fatal("no chunks")
	}
	unrelated := bigFile(rng, 4000)
	chunks = Merge3(base, unrelated, bigFile(rng, 4000))
	if len(chunks) == 0 {
		t.Fatal("no chunks")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("too slow: %v", elapsed)
	}
}

func mergedText(chunks []Chunk) (string, bool) {
	var out []string
	for _, c := range chunks {
		switch c.Kind {
		case ChunkStable, ChunkOurs:
			out = append(out, c.Ours...)
		case ChunkTheirs:
			out = append(out, c.Theirs...)
		default:
			return "", false
		}
	}
	return strings.Join(out, "\n") + "\n", true
}

func TestMerge3AgreesWithGitMergeFile(t *testing.T) {
	rng := rand.New(rand.NewSource(11))
	dir := t.TempDir()
	edit := func(base []string) []string {
		out := append([]string(nil), base...)
		for n := rng.Intn(3); n >= 0; n-- {
			if len(out) == 0 {
				break
			}
			i := rng.Intn(len(out))
			switch rng.Intn(3) {
			case 0:
				out[i] = "edit" + itoa(rng.Intn(100))
			case 1:
				out = append(out[:i], out[i+1:]...)
			default:
				out = append(out[:i], append([]string{"ins" + itoa(rng.Intn(100))}, out[i:]...)...)
			}
		}
		return out
	}
	clean, compared := 0, 0
	for trial := 0; trial < 60; trial++ {
		base := make([]string, 25)
		for i := range base {
			base[i] = "base" + itoa(i)
		}
		ours, theirs := edit(base), edit(base)
		chunks := Merge3(base, ours, theirs)
		mine, ok := mergedText(chunks)
		if !ok {
			continue
		}
		clean++
		files := map[string][]string{"base": base, "ours": ours, "theirs": theirs}
		for name, content := range files {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(strings.Join(content, "\n")+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		cmd := exec.Command("git", "merge-file", "-p", "ours", "base", "theirs")
		cmd.Dir = dir
		out, err := cmd.Output()
		if err != nil {
			continue
		}
		compared++
		if string(out) != mine {
			t.Fatalf("trial %d differs from git merge-file\nbase=%v\nours=%v\ntheirs=%v\nmine=%q\ngit=%q", trial, base, ours, theirs, mine, out)
		}
	}
	if clean == 0 || compared == 0 {
		t.Fatalf("no clean merges compared (%d clean, %d compared)", clean, compared)
	}
}

func TestSplitLines(t *testing.T) {
	if got := SplitLines(""); got != nil {
		t.Fatalf("empty: %v", got)
	}
	if got := SplitLines("a\n\nb\n"); strings.Join(got, "|") != "a||b" {
		t.Fatalf("got %q", got)
	}
	if got := SplitLines("a\nb"); len(got) != 2 {
		t.Fatalf("no trailing newline: %q", got)
	}
}
