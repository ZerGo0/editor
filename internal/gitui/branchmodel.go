package gitui

import (
	"sort"
	"strings"

	"github.com/ZerGo0/editor/internal/fuzzy"
	"github.com/ZerGo0/editor/internal/git"
)

const recentLimit = 5

type branchKind int

const (
	kindLocal branchKind = iota
	kindRemote
	kindTag
)

type branchEntry struct {
	header bool
	title  string
	group  string
	kind   branchKind
	name   string
	branch git.Branch
	tag    git.Tag
}

func (e branchEntry) key() string {
	if e.header {
		return "h:" + e.title
	}
	return e.group + ":" + e.name
}

func buildBranchEntries(branches []git.Branch, tags []git.Tag, query string) []branchEntry {
	var locals, remotes []git.Branch
	for _, b := range branches {
		switch {
		case strings.HasSuffix(b.Name, "/HEAD"):
		case b.Remote:
			remotes = append(remotes, b)
		default:
			locals = append(locals, b)
		}
	}
	sort.SliceStable(locals, func(i, j int) bool { return locals[i].Name < locals[j].Name })
	sort.SliceStable(remotes, func(i, j int) bool { return remotes[i].Name < remotes[j].Name })
	sortedTags := append([]git.Tag(nil), tags...)
	sort.SliceStable(sortedTags, func(i, j int) bool { return sortedTags[i].Name > sortedTags[j].Name })
	query = strings.TrimSpace(query)
	var out []branchEntry
	addBranches := func(title string, list []git.Branch, kind branchKind) {
		idx := matchOrder(query, len(list), func(i int) string { return list[i].Name })
		if len(idx) == 0 {
			return
		}
		out = append(out, branchEntry{header: true, title: title})
		for _, i := range idx {
			out = append(out, branchEntry{group: title, kind: kind, name: list[i].Name, branch: list[i]})
		}
	}
	if query == "" {
		var recent []git.Branch
		for _, b := range locals {
			if !b.Current && !b.Time.IsZero() {
				recent = append(recent, b)
			}
		}
		sort.SliceStable(recent, func(i, j int) bool { return recent[i].Time.After(recent[j].Time) })
		addBranches("Recent", recent[:min(len(recent), recentLimit)], kindLocal)
	}
	addBranches("Local", locals, kindLocal)
	addBranches("Remote", remotes, kindRemote)
	idx := matchOrder(query, len(sortedTags), func(i int) string { return sortedTags[i].Name })
	if len(idx) > 0 {
		out = append(out, branchEntry{header: true, title: "Tags"})
		for _, i := range idx {
			out = append(out, branchEntry{group: "Tags", kind: kindTag, name: sortedTags[i].Name, tag: sortedTags[i]})
		}
	}
	return out
}

func matchOrder(query string, n int, text func(int) string) []int {
	if query == "" {
		idx := make([]int, n)
		for i := range idx {
			idx[i] = i
		}
		return idx
	}
	res := fuzzy.Filter(query, n, text)
	idx := make([]int, len(res))
	for i, r := range res {
		idx[i] = r.Index
	}
	return idx
}
