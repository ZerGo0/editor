package prui

import (
	"cmp"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/github"
)

const (
	shortSHALength  = 7
	pullPathSegment = "pull"
	detailIDPrefix  = "pr"
	diffIDSegment   = "diff"
	createViewID    = "pr/new"

	statusAdded    = "added"
	statusRemoved  = "removed"
	statusRenamed  = "renamed"
	statusModified = "modified"

	reviewApproved         = "APPROVED"
	reviewChangesRequested = "CHANGES_REQUESTED"
	reviewCommented        = "COMMENTED"
	reviewDismissed        = "DISMISSED"
	reviewPending          = "PENDING"

	prStateOpen   = "Open"
	prStateDraft  = "Draft"
	prStateMerged = "Merged"
	prStateClosed = "Closed"

	conclusionSuccess = "success"
	conclusionNeutral = "neutral"
	conclusionSkipped = "skipped"
	conclusionStale   = "stale"
	statusCompleted   = "completed"
)

var preferredBases = []string{"main", "master", "develop", "trunk"}

type checkState int

const (
	checkUnknown checkState = iota
	checkPending
	checkSuccess
	checkFailure
)

func checkOutcome(c github.Check) checkState {
	if c.Status != statusCompleted {
		return checkPending
	}
	switch c.Conclusion {
	case conclusionSuccess, conclusionNeutral, conclusionSkipped:
		return checkSuccess
	case conclusionStale:
		return checkPending
	case "":
		return checkUnknown
	}
	return checkFailure
}

func summarizeChecks(checks []github.Check) checkState {
	result := checkUnknown
	for _, c := range checks {
		switch checkOutcome(c) {
		case checkFailure:
			return checkFailure
		case checkPending:
			result = checkPending
		case checkSuccess:
			if result == checkUnknown {
				result = checkSuccess
			}
		}
	}
	return result
}

func countChecks(checks []github.Check) (passed int) {
	for _, c := range checks {
		if checkOutcome(c) == checkSuccess {
			passed++
		}
	}
	return passed
}

func checkStateLabel(s checkState) string {
	switch s {
	case checkSuccess:
		return "Checks passed"
	case checkFailure:
		return "Checks failed"
	case checkPending:
		return "Checks running"
	}
	return ""
}

func prStateLabel(pr github.PullRequest) string {
	switch {
	case pr.Merged:
		return prStateMerged
	case pr.State == github.StateClosed:
		return prStateClosed
	case pr.Draft:
		return prStateDraft
	}
	return prStateOpen
}

func parsePRNumber(text string) (int, error) {
	text = strings.TrimSpace(text)
	if u, err := url.Parse(text); err == nil && u.Host != "" {
		segments := strings.Split(strings.Trim(u.Path, "/"), "/")
		if i := slices.Index(segments, pullPathSegment); i >= 0 && i+1 < len(segments) {
			text = segments[i+1]
		}
	}
	n, err := strconv.Atoi(strings.TrimPrefix(text, "#"))
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("%q is not a pull request number", text)
	}
	return n, nil
}

func detailID(repo github.Remote, number int) string {
	return fmt.Sprintf("%s/%s/%s/%d", detailIDPrefix, repo.Owner, repo.Name, number)
}

func detailTitle(number int, title string) string {
	if title == "" {
		return fmt.Sprintf("#%d", number)
	}
	return fmt.Sprintf("#%d %s", number, title)
}

func shortSHA(sha string) string {
	if len(sha) > shortSHALength {
		return sha[:shortSHALength]
	}
	return sha
}

func fileStatusLetter(status string) string {
	switch status {
	case statusAdded:
		return "A"
	case statusRemoved:
		return "D"
	case statusRenamed:
		return "R"
	case statusModified:
		return "M"
	}
	return "?"
}

func reviewStateLabel(state string) string {
	switch state {
	case reviewApproved:
		return "approved these changes"
	case reviewChangesRequested:
		return "requested changes"
	case reviewDismissed:
		return "review dismissed"
	case reviewCommented:
		return "reviewed"
	}
	return strings.ToLower(state)
}

func pendingReviewID(reviews []github.Review) string {
	for _, r := range reviews {
		if r.State == reviewPending {
			return r.ID
		}
	}
	return ""
}

func latestReviewStates(reviews []github.Review) map[string]string {
	latest := map[string]github.Review{}
	for _, r := range reviews {
		if r.State == reviewPending || r.State == reviewCommented || r.Author == "" {
			continue
		}
		if prev, ok := latest[r.Author]; !ok || r.Submitted.After(prev.Submitted) {
			latest[r.Author] = r
		}
	}
	states := make(map[string]string, len(latest))
	for author, r := range latest {
		states[author] = r.State
	}
	return states
}

type branchInfo struct {
	Name      string
	HeadRef   string
	HeadOwner string
	Remote    string
	Upstream  string
	Subject   string
	OID       string
}

func currentBranch(branches []git.Branch, repo github.Remote, byName map[string]github.Remote) branchInfo {
	for _, b := range branches {
		if b.Remote || !b.Current {
			continue
		}
		info := branchInfo{Name: b.Name, HeadRef: b.Name, HeadOwner: repo.Owner, Upstream: b.Upstream, Subject: b.Subject, OID: b.OID}
		for name, remote := range byName {
			rest, ok := strings.CutPrefix(b.Upstream, name+"/")
			if ok && rest != "" && len(name) > len(info.Remote) {
				info.Remote, info.HeadRef, info.HeadOwner = name, rest, remote.Owner
			}
		}
		return info
	}
	return branchInfo{}
}

func remoteBranchNames(branches []git.Branch, remote, exclude string) []string {
	prefix := remote + "/"
	var names []string
	for _, b := range branches {
		name, ok := strings.CutPrefix(b.Name, prefix)
		if !b.Remote || !ok || name == "" || name == exclude || slices.Contains(names, name) {
			continue
		}
		names = append(names, name)
	}
	slices.SortFunc(names, func(a, b string) int {
		return cmp.Or(cmp.Compare(baseRank(a), baseRank(b)), strings.Compare(a, b))
	})
	return names
}

func baseRank(name string) int {
	if i := slices.Index(preferredBases, name); i >= 0 {
		return i
	}
	return len(preferredBases)
}

func headSpec(repo github.Remote, b branchInfo) string {
	if b.HeadOwner == "" || b.HeadOwner == repo.Owner {
		return b.HeadRef
	}
	return b.HeadOwner + ":" + b.HeadRef
}

func localBranchNames(branches []git.Branch) []string {
	var names []string
	for _, b := range branches {
		if !b.Remote {
			names = append(names, b.Name)
		}
	}
	return names
}

func pullURL(repo github.Remote, number int) string {
	host := repo.Host
	if host == "" {
		host = github.DotComHost
	}
	return fmt.Sprintf("https://%s/%s/%s/pull/%d", host, repo.Owner, repo.Name, number)
}
