package session

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"slices"

	"github.com/ZerGo0/editor/internal/fsutil"
)

const (
	maxRecent = 20
	filePerm  = 0o600
)

type Tab struct {
	Path    string  `json:"path"`
	Preview bool    `json:"preview,omitempty"`
	Line    int     `json:"line,omitempty"`
	Col     int     `json:"col,omitempty"`
	ScrollY float64 `json:"scrollY,omitempty"`
	Pinned  bool    `json:"pinned,omitempty"`
}

type Group struct {
	Tabs   []Tab `json:"tabs"`
	Active int   `json:"active"`
}

type Node struct {
	Group    *Group  `json:"group,omitempty"`
	Vertical bool    `json:"vertical,omitempty"`
	Percent  float32 `json:"percent,omitempty"`
	First    *Node   `json:"first,omitempty"`
	Second   *Node   `json:"second,omitempty"`
}

type Project struct {
	SidebarWidth  float32         `json:"sidebarWidth,omitempty"`
	SidebarHidden bool            `json:"sidebarHidden,omitempty"`
	Expanded      []string        `json:"expanded,omitempty"`
	Layout        *Node           `json:"layout,omitempty"`
	ActiveGroup   int             `json:"activeGroup,omitempty"`
	SideView      string          `json:"sideView,omitempty"`
	PanelOpen     bool            `json:"panelOpen,omitempty"`
	Panel         string          `json:"panel,omitempty"`
	PanelPercent  float32         `json:"panelPercent,omitempty"`
	Search        json.RawMessage `json:"search,omitempty"`
}

type State struct {
	Recent   []string           `json:"recent"`
	Open     []string           `json:"open"`
	Projects map[string]Project `json:"projects"`
}

type Store struct {
	path  string
	State State
}

func Load(path string) (*Store, error) {
	s := &Store{path: path, State: State{Projects: map[string]Project{}}}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(data, &s.State); err != nil {
		return s, err
	}
	if s.State.Projects == nil {
		s.State.Projects = map[string]Project{}
	}
	return s, nil
}

func (s *Store) Save() error {
	s.prune()
	data, err := json.MarshalIndent(s.State, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(s.path, data, filePerm)
}

func (s *Store) prune() {
	for root := range s.State.Projects {
		if !slices.Contains(s.State.Recent, root) && !slices.Contains(s.State.Open, root) {
			delete(s.State.Projects, root)
		}
	}
}

func (s *Store) AddRecent(root string) {
	recent := slices.DeleteFunc(s.State.Recent, func(r string) bool { return r == root })
	s.State.Recent = slices.Insert(recent, 0, root)
	if len(s.State.Recent) > maxRecent {
		s.State.Recent = s.State.Recent[:maxRecent]
	}
}

func (s *Store) RemoveRecent(root string) {
	s.State.Recent = slices.DeleteFunc(s.State.Recent, func(r string) bool { return r == root })
}
