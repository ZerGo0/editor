package review

import "fmt"

type DiffMode string

const (
	DiffUncommitted DiffMode = "uncommitted"
	DiffStaged      DiffMode = "staged"
	DiffUnstaged    DiffMode = "unstaged"
	DiffLastCommit  DiffMode = "last-commit"
	DiffBranch      DiffMode = "branch"
)

var DiffModes = []DiffMode{DiffUncommitted, DiffStaged, DiffUnstaged, DiffLastCommit, DiffBranch}

var diffTitles = map[DiffMode]string{
	DiffUncommitted: "Uncommitted Changes",
	DiffStaged:      "Staged Changes",
	DiffUnstaged:    "Unstaged Changes",
	DiffLastCommit:  "Last Commit",
	DiffBranch:      "Branch Diff",
}

func (m DiffMode) Valid() bool {
	_, ok := diffTitles[m]
	return ok
}

func (m DiffMode) Title() string { return diffTitles[m] }

func DescribeDiff(mode DiffMode, base string) string {
	switch mode {
	case DiffUncommitted:
		return "Uncommitted changes"
	case DiffStaged:
		return "Staged changes"
	case DiffUnstaged:
		return "Unstaged changes"
	case DiffLastCommit:
		return "Last commit"
	case DiffBranch:
		if base == "" {
			return "Branch diff"
		}
		return fmt.Sprintf("Branch diff vs `%s`", base)
	}
	return string(mode)
}
