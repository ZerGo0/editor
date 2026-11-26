package issueui

import (
	"regexp"
	"strconv"
	"strings"
)

const (
	varUser           = "user"
	varOwner          = "owner"
	varRepository     = "repository"
	varIssueNumber    = "issueNumber"
	varNumberLabel    = "issueNumberLabel"
	varIssueTitle     = "issueTitle"
	varSanitizedTitle = "sanitizedIssueTitle"

	maxTitleSlugLength = 50
	slugSeparator      = "-"
	pathSeparator      = "/"
	refLockSuffix      = ".lock"
	fallbackBranchFmt  = "issue-"
	invalidRefChars    = " ~^:?*[\\\x7f"
	refAtBrace         = "@{"
	refDoubleDot       = ".."
)

var templateVar = regexp.MustCompile(`\$\{(\w+)\}`)

type templateVars struct {
	User, Owner, Repo string
	Number            int
	Title             string
}

func expandTemplate(tpl string, values map[string]string) string {
	return templateVar.ReplaceAllStringFunc(tpl, func(match string) string {
		name := match[2 : len(match)-1]
		if v, ok := values[name]; ok {
			return v
		}
		return match
	})
}

func (v templateVars) values(titleValue string) map[string]string {
	return map[string]string{
		varUser:           v.User,
		varOwner:          v.Owner,
		varRepository:     v.Repo,
		varIssueNumber:    strconv.Itoa(v.Number),
		varNumberLabel:    "#" + strconv.Itoa(v.Number),
		varIssueTitle:     titleValue,
		varSanitizedTitle: sanitizeTitle(v.Title),
	}
}

func branchName(tpl string, v templateVars) string {
	name := sanitizeBranch(expandTemplate(tpl, v.values(sanitizeTitle(v.Title))))
	if name == "" {
		return fallbackBranchFmt + strconv.Itoa(v.Number)
	}
	return name
}

func commitMessage(tpl string, v templateVars) string {
	return expandTemplate(tpl, v.values(v.Title))
}

func sanitizeTitle(title string) string {
	var b strings.Builder
	pendingSeparator := false
	for _, r := range strings.ToLower(title) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r > 0x7f {
			if pendingSeparator && b.Len() > 0 {
				b.WriteString(slugSeparator)
			}
			pendingSeparator = false
			b.WriteRune(r)
			continue
		}
		pendingSeparator = true
	}
	slug := []rune(b.String())
	if len(slug) > maxTitleSlugLength {
		slug = slug[:maxTitleSlugLength]
	}
	return strings.Trim(string(slug), slugSeparator)
}

func sanitizeBranch(name string) string {
	var b strings.Builder
	for _, r := range name {
		if r < ' ' || strings.ContainsRune(invalidRefChars, r) {
			r = '-'
		}
		b.WriteRune(r)
	}
	name = strings.ReplaceAll(b.String(), refAtBrace, slugSeparator)
	for strings.Contains(name, refDoubleDot) {
		name = strings.ReplaceAll(name, refDoubleDot, ".")
	}
	var parts []string
	for _, part := range strings.Split(name, pathSeparator) {
		part = strings.TrimLeft(part, ".-")
		for strings.HasSuffix(part, refLockSuffix) {
			part = strings.TrimSuffix(part, refLockSuffix)
		}
		part = strings.TrimRight(part, ".")
		if part != "" && part != "@" {
			parts = append(parts, part)
		}
	}
	return strings.Join(parts, pathSeparator)
}
