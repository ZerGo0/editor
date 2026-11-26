package issueui

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/ZerGo0/editor/internal/github"
)

const (
	storeDirName   = "issues"
	storeKeyLength = 16
	storeFileExt   = ".json"
	storeDirPerm   = 0o755
	storeFilePerm  = 0o644
	maxRecentWork  = 10
)

type workItem struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	URL    string `json:"url,omitempty"`
	Branch string `json:"branch"`
	Base   string `json:"base,omitempty"`
}

type workState struct {
	Active *workItem  `json:"active,omitempty"`
	Recent []workItem `json:"recent,omitempty"`
}

func storePath(dataDir string, repo github.Remote) string {
	sum := sha256.Sum256([]byte(repo.Host + "/" + repo.Owner + "/" + repo.Name))
	return filepath.Join(dataDir, storeDirName, hex.EncodeToString(sum[:])[:storeKeyLength]+storeFileExt)
}

func loadWork(dataDir string, repo github.Remote) (workState, error) {
	if dataDir == "" {
		return workState{}, nil
	}
	data, err := os.ReadFile(storePath(dataDir, repo))
	if errors.Is(err, fs.ErrNotExist) {
		return workState{}, nil
	}
	if err != nil {
		return workState{}, fmt.Errorf("read working issue state: %w", err)
	}
	var state workState
	if err := json.Unmarshal(data, &state); err != nil {
		return workState{}, fmt.Errorf("decode working issue state: %w", err)
	}
	return state.valid(), nil
}

func saveWork(dataDir string, repo github.Remote, state workState) error {
	if dataDir == "" {
		return nil
	}
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("encode working issue state: %w", err)
	}
	path := storePath(dataDir, repo)
	if err := os.MkdirAll(filepath.Dir(path), storeDirPerm); err != nil {
		return fmt.Errorf("create working issue state directory: %w", err)
	}
	if err := os.WriteFile(path, data, storeFilePerm); err != nil {
		return fmt.Errorf("write working issue state: %w", err)
	}
	return nil
}

func (w workItem) usable() bool {
	return w.Number > 0 && w.Branch != ""
}

func (s workState) valid() workState {
	var out workState
	if s.Active != nil && s.Active.usable() {
		active := *s.Active
		out.Active = &active
	}
	for _, item := range s.Recent {
		if item.usable() && len(out.Recent) < maxRecentWork {
			out.Recent = append(out.Recent, item)
		}
	}
	return out
}

func (s *workState) activate(item workItem) {
	s.remember(item)
	s.Active = &item
}

func (s *workState) deactivate() {
	if s.Active != nil {
		s.remember(*s.Active)
	}
	s.Active = nil
}

func (s *workState) remember(item workItem) {
	recent := []workItem{item}
	for _, old := range s.Recent {
		if old.Number != item.Number && len(recent) < maxRecentWork {
			recent = append(recent, old)
		}
	}
	s.Recent = recent
}

func (s workState) previous(number int) (workItem, bool) {
	for _, item := range s.Recent {
		if item.Number == number {
			return item, true
		}
	}
	return workItem{}, false
}
