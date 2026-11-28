package list

import (
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
)

const accessSeparator = ", "

type accessLabelID struct{}

func AccessName(it Item) string {
	parts := make([]string, 0, 5)
	if it.Label != "" {
		parts = append(parts, it.Label)
	}
	for _, s := range []string{it.Git.Name(), it.Description, it.Aside} {
		if s != "" && !strings.Contains(it.Label, s) {
			parts = append(parts, s)
		}
	}
	if it.Count > 0 {
		parts = append(parts, strconv.Itoa(it.Count))
	}
	return strings.Join(parts, accessSeparator)
}

func defaultAccessLabels(l ui.Element, s *ui.ListState, item func(i int) Item) {
	owned := ui.Local(l, accessLabelID{}, func() bool { return s.Label == nil })
	if *owned {
		s.Label = func(i int) string { return AccessName(item(i)) }
	}
}
