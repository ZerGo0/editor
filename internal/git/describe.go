package git

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

const (
	describeMaxLines = 40
	remoteFallback   = "the remote"
	carriageReturn   = "\r"
)

const (
	unavailableText = "Git is not installed. Install Git or the Xcode Command Line Tools (xcode-select --install), then check again."
	damagedText     = "This folder has a .git folder, but Git cannot read it. The repository may be damaged."
	lockText        = "Another Git process seems to be running in this repository. If no Git process is running, delete %s and try again."
	signInText      = "Git could not sign in to %s. Set up a credential helper (for example, run gh auth setup-git) or use an SSH remote URL."
	sshKeyText      = "Git could not sign in to %s with SSH. Add your SSH key to ssh-agent and to your account on %s."
	offlineText     = "Cannot reach %s. Check your network connection and try again."
	ownershipText   = "Git does not trust this folder because another user owns it. To trust it, run git config --global --add safe.directory with the folder path."
	exitText        = "Git stopped with exit code %d and printed no message."
)

var (
	gitDirPath   = regexp.MustCompile(`/(?:[^\s'"/]+/)*\.git/`)
	lockPath     = regexp.MustCompile(`Unable to create '([^']+\.lock)': File exists`)
	quotedURL    = regexp.MustCompile(`'((?:https?|ssh|git)://[^']+)'`)
	resolveHost  = regexp.MustCompile(`Could not resolve host: ([^\s']+)`)
	sshHost      = regexp.MustCompile(`(?:[\w.-]+@)?([\w.-]+): Permission denied \(publickey`)
	signInMarker = []string{"could not read Username", "could not read Password", "Authentication failed", "terminal prompts disabled", "Invalid username or password", "HTTP Basic: Access denied"}
	offlineMark  = []string{"Could not resolve host", "Failed to connect", "Connection refused", "Connection timed out", "Operation timed out", "Network is unreachable", "Could not read from remote repository"}
)

func Describe(err error) (string, bool) {
	if err == nil {
		return "", false
	}
	if errors.Is(err, ErrUnavailable) {
		return unavailableText, true
	}
	var ge *Error
	if !errors.As(err, &ge) {
		return "", false
	}
	out := cleanOutput(ge.Stderr, ge.Dir)
	hint := hintFor(ge.Stderr, errors.Is(err, ErrDamaged))
	switch {
	case out == "" && hint == "":
		return fmt.Sprintf(exitText, ge.ExitCode), true
	case out == "":
		return hint, true
	case hint == "":
		return out, true
	}
	return hint + "\n\n" + out, true
}

func hintFor(stderr string, damaged bool) string {
	switch {
	case damaged:
		return damagedText
	case lockPath.MatchString(stderr):
		return fmt.Sprintf(lockText, shortenPaths(lockPath.FindStringSubmatch(stderr)[1], ""))
	case strings.Contains(stderr, "dubious ownership"):
		return ownershipText
	case strings.Contains(stderr, xcrunMarker):
		return unavailableText
	case containsAny(stderr, signInMarker):
		return fmt.Sprintf(signInText, remoteHost(stderr))
	case sshHost.MatchString(stderr):
		host := sshHost.FindStringSubmatch(stderr)[1]
		return fmt.Sprintf(sshKeyText, host, host)
	case containsAny(stderr, offlineMark):
		return fmt.Sprintf(offlineText, remoteHost(stderr))
	}
	return ""
}

func remoteHost(stderr string) string {
	if m := resolveHost.FindStringSubmatch(stderr); m != nil {
		return m[1]
	}
	if m := quotedURL.FindStringSubmatch(stderr); m != nil {
		if u, err := url.Parse(m[1]); err == nil && u.Hostname() != "" {
			return u.Hostname()
		}
	}
	return remoteFallback
}

func cleanOutput(stderr, dir string) string {
	var lines []string
	for line := range strings.SplitSeq(stderr, "\n") {
		if i := strings.LastIndex(line, carriageReturn); i >= 0 {
			line = line[i+1:]
		}
		line = strings.TrimRight(line, " \t")
		if _, _, ok := ParseProgress(line); ok || strings.TrimSpace(line) == "" {
			continue
		}
		lines = append(lines, shortenPaths(line, dir))
	}
	if over := len(lines) - describeMaxLines; over > 0 {
		lines = append([]string{"(" + strconv.Itoa(over) + " earlier lines not shown)"}, lines[over:]...)
	}
	return strings.Join(lines, "\n")
}

func shortenPaths(s, dir string) string {
	s = gitDirPath.ReplaceAllString(s, gitEntry+"/")
	if dir != "" {
		s = strings.ReplaceAll(s, strings.TrimSuffix(dir, "/")+"/", "")
	}
	return s
}
