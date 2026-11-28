package session

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"slices"

	"github.com/ZerGo0/editor/internal/fsutil"
)

const (
	maxRecent      = 20
	maxRecentFiles = 50
	filePerm       = 0o600
)

type Tab struct {
	Path       string   `json:"path"`
	Title      string   `json:"title,omitempty"`
	Preview    bool     `json:"preview,omitempty"`
	Line       int      `json:"line,omitempty"`
	Col        int      `json:"col,omitempty"`
	ScrollY    float64  `json:"scrollY,omitempty"`
	Selections [][2]int `json:"selections,omitempty"`
	Folds      []int    `json:"folds,omitempty"`
	Pinned     bool     `json:"pinned,omitempty"`
}

type Location struct {
	Path string `json:"path"`
	Line int    `json:"line,omitempty"`
	Col  int    `json:"col,omitempty"`
}

type Group struct {
	Tabs    []Tab      `json:"tabs"`
	Active  int        `json:"active"`
	Back    []Location `json:"back,omitempty"`
	Forward []Location `json:"forward,omitempty"`
	Recent  []string   `json:"recent,omitempty"`
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
	PanelRight    bool            `json:"panelRight,omitempty"`
	OpenEditors   bool            `json:"openEditors,omitempty"`
	Collapsed     []string        `json:"collapsed,omitempty"`
	HiddenViews   []string        `json:"hiddenViews,omitempty"`
	StatusHidden  []string        `json:"statusHidden,omitempty"`
	LastEdit      *Location       `json:"lastEdit,omitempty"`
	RecentFiles   []string        `json:"recentFiles,omitempty"`
	PanelSide     float32         `json:"panelSidePercent,omitempty"`
	PanelMax      bool            `json:"panelMaximized,omitempty"`
	TreeTop       int             `json:"treeTop,omitempty"`
	ClosedTabs    []Tab           `json:"closedTabs,omitempty"`
	Views         Views           `json:"views,omitempty"`
}

type Views = map[string]json.RawMessage

type State struct {
	Recent   []string           `json:"recent"`
	Files    []string           `json:"recentFiles,omitempty"`
	Open     []string           `json:"open"`
	Projects map[string]Project `json:"projects"`
	Commands []string           `json:"commands,omitempty"`
	Views    Views              `json:"views,omitempty"`
}

type Store struct {
	path    string
	written []byte
	State   State
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
	data, err := json.Marshal(s.State)
	if err != nil {
		return err
	}
	if bytes.Equal(data, s.written) {
		return nil
	}
	if err := fsutil.WriteFileAtomic(s.path, data, filePerm); err != nil {
		return err
	}
	s.written = data
	return nil
}

func (s *Store) prune() {
	for root := range s.State.Projects {
		if !slices.Contains(s.State.Recent, root) && !slices.Contains(s.State.Open, root) {
			delete(s.State.Projects, root)
		}
	}
}

func (s *Store) AddRecent(root string) {
	s.State.Recent = pushFront(s.State.Recent, root, maxRecent)
}

func (s *Store) RemoveRecent(root string) {
	s.State.Recent = slices.DeleteFunc(s.State.Recent, func(r string) bool { return r == root })
}

func (s *Store) AddRecentFile(path string) {
	s.State.Files = pushFront(s.State.Files, path, maxRecentFiles)
}

func (s *Store) RemoveRecentFile(path string) {
	s.State.Files = slices.DeleteFunc(s.State.Files, func(p string) bool { return p == path })
}

func pushFront(list []string, item string, limit int) []string {
	list = slices.Insert(slices.DeleteFunc(list, func(s string) bool { return s == item }), 0, item)
	return list[:min(len(list), limit)]
}
