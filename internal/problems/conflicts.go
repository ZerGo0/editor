package problems

import "strings"

const (
	conflictStart   = "<<<<<<< "
	conflictDivider = "======="
	conflictEnd     = ">>>>>>> "
	conflictWidth   = 7
	conflictMessage = "Merge conflict marker"
)

func checkConflicts(text string) []Problem {
	if !strings.Contains(text, conflictDivider) {
		return nil
	}
	var starts, dividers, ends []Problem
	eachLine(text, func(no int, line string) bool {
		var target *[]Problem
		switch {
		case strings.HasPrefix(line, conflictStart):
			target = &starts
		case line == conflictDivider:
			target = &dividers
		case strings.HasPrefix(line, conflictEnd):
			target = &ends
		default:
			return true
		}
		*target = append(*target, Problem{Line: no, EndCol: conflictWidth, Severity: Error, Message: conflictMessage, Source: sourceGit})
		return true
	})
	if len(starts) == 0 || len(dividers) == 0 || len(ends) == 0 {
		return nil
	}
	out := append(append(starts, dividers...), ends...)
	return out[:min(len(out), MaxProblemsPerKind)]
}
