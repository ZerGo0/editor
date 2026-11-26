package filetree

import (
	"path"
	"path/filepath"

	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/git"
)

const (
	codeAdded    = 'A'
	codeModified = 'M'
	codeDeleted  = 'D'
	codeRenamed  = 'R'
	codeCopied   = 'C'
	codeTypeEdit = 'T'
	codeUnknown  = '?'
)

var folderPriority = []token.GitStatus{token.GitConflict, token.GitModified, token.GitDeleted, token.GitAdded, token.GitRenamed, token.GitCopied, token.GitUntracked}

type gitState struct {
	files map[string]token.GitStatus
	dirs  map[string]token.GitStatus
}

func StatusOf(f git.FileStatus) token.GitStatus {
	switch {
	case f.Conflict:
		return token.GitConflict
	case f.Index == codeUnknown:
		return token.GitUntracked
	case f.WorkTree == codeDeleted || f.Index == codeDeleted:
		return token.GitDeleted
	case f.WorkTree == codeModified || f.WorkTree == codeTypeEdit:
		return token.GitModified
	case f.Index == codeAdded:
		return token.GitAdded
	case f.Index == codeRenamed:
		return token.GitRenamed
	case f.Index == codeCopied:
		return token.GitCopied
	case f.Index == codeModified || f.Index == codeTypeEdit:
		return token.GitModified
	}
	return token.GitNone
}

func (t *Tree) SetGit(files []git.FileStatus) {
	st := gitState{files: map[string]token.GitStatus{}, dirs: map[string]token.GitStatus{}}
	for _, f := range files {
		s := StatusOf(f)
		if s == token.GitNone {
			continue
		}
		rel := path.Clean(f.Path)
		st.files[rel] = s
		if s == token.GitDeleted {
			continue
		}
		for dir := path.Dir(rel); dir != "." && dir != "/"; dir = path.Dir(dir) {
			if rank(s) < rank(st.dirs[dir]) {
				st.dirs[dir] = s
			}
		}
	}
	t.git = st
}

func rank(s token.GitStatus) int {
	for i, p := range folderPriority {
		if p == s {
			return i
		}
	}
	return len(folderPriority)
}

func (t *Tree) gitStatus(p string) token.GitStatus {
	rel, err := filepath.Rel(t.Root, p)
	if err != nil || !filepath.IsLocal(rel) {
		return token.GitNone
	}
	rel = filepath.ToSlash(rel)
	if t.isDir[p] {
		return t.git.dirs[rel]
	}
	return t.git.files[rel]
}
