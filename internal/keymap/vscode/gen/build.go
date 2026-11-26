package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"go/format"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/ZerGo0/editor/internal/jsonc"
	"github.com/ZerGo0/editor/internal/keymap/vscode"
)

const (
	tableFile    = "tables_gen.go"
	filePerm     = 0o644
	removePrefix = "-"
	strokeSep    = " "
	modifierSep  = "+"
)

var (
	versionPattern = regexp.MustCompile(`Default Keybindings of Visual Studio Code (\S+) for`)
	preferred      = map[string]bool{"workbench.action.splitEditorOrthogonal": true}
)

type rawEntry struct {
	Key     string          `json:"key"`
	Command string          `json:"command"`
	When    string          `json:"when"`
	Args    json.RawMessage `json:"args"`
}

type platformTable struct {
	platform
	entries []vscode.Entry
}

type table struct {
	version   string
	platforms []platformTable
}

func parseCommands(text string) (map[string]bool, error) {
	supported := map[string]bool{}
	sc := bufio.NewScanner(strings.NewReader(text))
	for sc.Scan() {
		id := strings.TrimSpace(sc.Text())
		if id == "" {
			continue
		}
		if supported[id] {
			return nil, fmt.Errorf("commands.txt lists %q twice", id)
		}
		supported[id] = true
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	for _, c := range vscode.EditorCommands {
		supported[c.ID] = true
	}
	for id := range vscode.EditorAliases {
		supported[id] = true
	}
	for id := range vscode.WorkbenchAliases {
		supported[id] = true
	}
	return supported, nil
}

func build(dumps [][]byte, supported map[string]bool) (*table, error) {
	if len(dumps) != len(platforms) {
		return nil, fmt.Errorf("got %d dumps, want %d", len(dumps), len(platforms))
	}
	t := &table{}
	for i, p := range platforms {
		version, raw, err := parseDump(dumps[i])
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p.file, err)
		}
		if t.version != "" && version != t.version {
			return nil, fmt.Errorf("%s: VS Code %s, other dumps %s", p.file, version, t.version)
		}
		t.version = version
		entries, err := filter(raw, supported)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p.file, err)
		}
		t.platforms = append(t.platforms, platformTable{platform: p, entries: entries})
	}
	return t, nil
}

func parseDump(data []byte) (string, []rawEntry, error) {
	m := versionPattern.FindSubmatch(data)
	if m == nil {
		return "", nil, fmt.Errorf("no VS Code version in the header")
	}
	var raw []rawEntry
	if err := json.Unmarshal(jsonc.Standard(data), &raw); err != nil {
		return "", nil, fmt.Errorf("parse: %w", err)
	}
	for i, e := range raw {
		if strings.TrimSpace(e.Key) == "" || strings.TrimSpace(e.Command) == "" {
			return "", nil, fmt.Errorf("entry %d: needs a key and a command", i+1)
		}
	}
	return string(m[1]), raw, nil
}

func filter(raw []rawEntry, supported map[string]bool) ([]vscode.Entry, error) {
	var out []vscode.Entry
	for _, e := range raw {
		if !supported[strings.TrimPrefix(e.Command, removePrefix)] {
			continue
		}
		args, err := compactArgs(e.Args)
		if err != nil {
			return nil, fmt.Errorf("%s %s: %w", e.Key, e.Command, err)
		}
		out = append(out, vscode.Entry{Key: e.Key, Command: e.Command, When: e.When, Args: args})
	}
	return resolveConflicts(out)
}

func compactArgs(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", nil
	}
	var b bytes.Buffer
	if err := json.Compact(&b, raw); err != nil {
		return "", fmt.Errorf("args: %w", err)
	}
	return b.String(), nil
}

func resolveConflicts(entries []vscode.Entry) ([]vscode.Entry, error) {
	unconditional := map[string][]int{}
	for i, e := range entries {
		if e.When == "" {
			chord := normalizeChord(e.Key)
			unconditional[chord] = append(unconditional[chord], i)
		}
	}
	drop := map[int]bool{}
	for _, chord := range slices.Sorted(maps.Keys(unconditional)) {
		at := unconditional[chord]
		commands := map[string]bool{}
		for _, i := range at {
			commands[canonical(entries[i].Command)] = true
		}
		if len(commands) < 2 {
			continue
		}
		keep := slices.IndexFunc(at, func(i int) bool { return preferred[entries[i].Command] })
		if keep < 0 {
			return nil, fmt.Errorf("%s runs %d commands with no when clause; add the one VS Code runs to preferred", chord, len(commands))
		}
		for j, i := range at {
			if j != keep {
				drop[i] = true
			}
		}
	}
	var out []vscode.Entry
	for i, e := range entries {
		if !drop[i] {
			out = append(out, e)
		}
	}
	return out, nil
}

func canonical(id string) string {
	if to, ok := vscode.WorkbenchAliases[id]; ok {
		return to
	}
	if cmd, ok := vscode.EditorAliases[id]; ok {
		for _, c := range vscode.EditorCommands {
			if c.Command == cmd {
				return c.ID
			}
		}
	}
	return id
}

func normalizeChord(key string) string {
	strokes := strings.Fields(strings.ToLower(key))
	for i, s := range strokes {
		parts := strings.Split(s, modifierSep)
		mods := parts[:len(parts)-1]
		slices.Sort(mods)
		strokes[i] = strings.Join(append(mods, parts[len(parts)-1]), modifierSep)
	}
	return strings.Join(strokes, strokeSep)
}

func render(t *table) ([]byte, error) {
	var b bytes.Buffer
	b.WriteString("// Code generated by internal/keymap/vscode/gen. DO NOT EDIT.\n\npackage vscode\n\n")
	fmt.Fprintf(&b, "const (\n\tVersion = %s\n\tSourceRepo = %s\n\tSourceCommit = %s\n)\n", strconv.Quote(t.version), strconv.Quote(sourceRepo), strconv.Quote(sourceCommit))
	for _, p := range t.platforms {
		fmt.Fprintf(&b, "\nvar %s = []Entry{\n", p.variable)
		for _, e := range p.entries {
			fmt.Fprintf(&b, "\t{Key: %s, Command: %s", strconv.Quote(e.Key), strconv.Quote(e.Command))
			if e.When != "" {
				fmt.Fprintf(&b, ", When: %s", strconv.Quote(e.When))
			}
			if e.Args != "" {
				fmt.Fprintf(&b, ", Args: %s", strconv.Quote(e.Args))
			}
			b.WriteString("},\n")
		}
		b.WriteString("}\n")
	}
	return format.Source(b.Bytes())
}
