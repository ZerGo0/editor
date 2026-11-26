package git

import (
	"context"
	"fmt"
	"strings"
)

const (
	submoduleStatusChars = " -+U"
	submoduleMinLine     = 3
	describeOpen         = " ("
)

type Submodule struct {
	Path   string
	OID    string
	Status byte
}

func (r *Repo) Submodules(ctx context.Context) ([]Submodule, error) {
	out, err := r.readText(ctx, "submodule", "status")
	if err != nil {
		return nil, err
	}
	return parseSubmodules(out)
}

func parseSubmodules(text string) ([]Submodule, error) {
	var subs []Submodule
	for _, line := range strings.Split(text, "\n") {
		if line == "" {
			continue
		}
		if len(line) < submoduleMinLine || !strings.ContainsRune(submoduleStatusChars, rune(line[0])) {
			return nil, fmt.Errorf("git submodule status: malformed line %q", line)
		}
		oid, path, ok := strings.Cut(line[1:], " ")
		if !ok || oid == "" || path == "" {
			return nil, fmt.Errorf("git submodule status: malformed line %q", line)
		}
		if strings.HasSuffix(path, ")") {
			if i := strings.LastIndex(path, describeOpen); i > 0 {
				path = path[:i]
			}
		}
		subs = append(subs, Submodule{Path: path, OID: oid, Status: line[0]})
	}
	return subs, nil
}

func (r *Repo) UpdateSubmodules(ctx context.Context, initialize, recursive bool) error {
	args := []string{"submodule", "update"}
	if initialize {
		args = append(args, "--init")
	}
	if recursive {
		args = append(args, "--recursive")
	}
	_, err := r.write(ctx, args...)
	return err
}
