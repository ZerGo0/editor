package github

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrRateLimited = errors.New("github: rate limited")

type APIError struct {
	Status  int
	Message string
	Method  string
	URL     string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("github: %s %s: status %d: %s", e.Method, e.URL, e.Status, e.Message)
}

type RateLimitError struct {
	Status  int
	Message string
	Reset   time.Time
}

func (e *RateLimitError) Error() string {
	if e.Reset.IsZero() {
		return fmt.Sprintf("github: rate limited (status %d): %s", e.Status, e.Message)
	}
	return fmt.Sprintf("github: rate limited (status %d), resets at %s: %s", e.Status, e.Reset.Format(time.RFC3339), e.Message)
}

func (e *RateLimitError) Is(target error) bool { return target == ErrRateLimited }

type GraphQLMessage struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

type GraphQLError struct {
	Errors []GraphQLMessage
}

func (e *GraphQLError) Error() string {
	messages := make([]string, 0, len(e.Errors))
	for _, m := range e.Errors {
		messages = append(messages, m.Message)
	}
	return "github: graphql: " + strings.Join(messages, "; ")
}
