package errtext

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/url"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/ZerGo0/editor/internal/github"
)

type reasonErr struct{}

func (reasonErr) Error() string  { return "raw" }
func (reasonErr) Reason() string { return "Friendly." }

func TestText(t *testing.T) {
	tmp := "/private/var/folders/x/sub/.e.txt.2672547674.tmp"
	cases := []struct {
		err  error
		want string
	}{
		{&fs.PathError{Op: "save", Path: tmp, Err: syscall.EACCES}, "You do not have permission to change it. The file or its folder may be read-only."},
		{&fs.PathError{Op: "open", Path: "/a/b", Err: syscall.EACCES}, "You do not have permission to open it."},
		{&fs.PathError{Op: "stat", Path: "/a/b", Err: syscall.ENOENT}, "It does not exist or was moved."},
		{&fs.PathError{Op: "save", Path: "/a/b", Err: syscall.ENOENT}, "Its folder does not exist or was moved."},
		{fmt.Errorf("write: %w", &fs.PathError{Op: "write", Path: tmp, Err: syscall.ENOSPC}), "The disk is full. Free up space and try again."},
		{&fs.PathError{Op: "open", Path: "/a", Err: syscall.EINVAL}, "Invalid argument."},
		{fmt.Errorf("wrapped: %w", reasonErr{}), "Friendly."},
		{context.Canceled, "The operation was canceled."},
		{&url.Error{Op: "Get", URL: "http://x", Err: &net.OpError{Op: "dial", Err: syscall.ECONNREFUSED}}, unreachable},
		{&url.Error{Op: "Get", URL: "http://x", Err: &net.DNSError{Name: "x"}}, noServer},
		{&github.APIError{Status: 401}, "GitHub did not accept your sign-in. Sign in again with gh auth login."},
		{&github.APIError{Status: 422, Message: "Validation Failed"}, "GitHub rejected the request: Validation Failed."},
		{errors.New("prui: pull request 5 not found"), "Pull request 5 not found."},
		{errors.New(""), "Something went wrong."},
	}
	for _, c := range cases {
		got := Text(c.err)
		if got != c.want {
			t.Errorf("Text(%v) = %q, want %q", c.err, got, c.want)
		}
		if strings.Contains(got, ".tmp") || strings.Contains(got, "/") {
			t.Errorf("Text(%v) = %q shows a path", c.err, got)
		}
	}
}

func TestRateLimitText(t *testing.T) {
	now := time.Date(2026, 1, 1, 15, 0, 0, 0, time.UTC)
	for _, c := range []struct {
		reset time.Time
		want  string
	}{
		{time.Time{}, "GitHub rate limit reached. Try again later."},
		{now.Add(20 * time.Second), "GitHub rate limit reached. Try again in a minute."},
		{now.Add(11*time.Minute + time.Second), "GitHub rate limit reached. Try again in 12 minutes."},
	} {
		if got := rateLimitText(c.reset, now); got != c.want {
			t.Errorf("rateLimitText(%v) = %q, want %q", c.reset, got, c.want)
		}
	}
}
