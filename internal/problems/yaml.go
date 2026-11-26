package problems

import (
	"strings"

	"github.com/egoist/mygo/ui/editor"
)

const yamlLanguage = "YAML"

func checkYAML(lang *editor.Language, text string) []Problem {
	if lang == nil || lang.Name != yamlLanguage || !strings.Contains(text, "\t") {
		return nil
	}
	var out []Problem
	eachLine(text, func(no int, line string) bool {
		trimmed := strings.TrimLeft(line, " \t")
		indent := line[:len(line)-len(trimmed)]
		tab := strings.IndexByte(indent, '\t')
		if tab < 0 || trimmed == "" || strings.HasPrefix(trimmed, "#") {
			return true
		}
		out = append(out, Problem{Line: no, Col: tab, EndCol: len(indent), Severity: Warning, Message: "Tab used for indentation", Source: sourceYAML})
		return len(out) < MaxProblemsPerKind
	})
	return out
}
