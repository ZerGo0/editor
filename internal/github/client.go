package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	DotComHost           = "github.com"
	dotComRESTURL        = "https://api.github.com"
	dotComGraphQLURL     = "https://api.github.com/graphql"
	enterpriseRESTFmt    = "https://%s/api/v3"
	enterpriseGraphQLFmt = "https://%s/api/graphql"
	graphQLSuffix        = "/graphql"
	requestTimeout       = 30 * time.Second
	dialTimeout          = 10 * time.Second
	tlsTimeout           = 10 * time.Second
	maxResponseBytes     = 64 << 20
	maxPages             = 50
	userAgent            = "editor-github-client"
	apiVersion           = "2022-11-28"
	acceptJSON           = "application/vnd.github+json"
	graphQLErrRateLimit  = "RATE_LIMITED"
	linkRelNext          = `rel="next"`
)

type Client struct {
	host       string
	token      string
	http       *http.Client
	restURL    string
	graphqlURL string
	cache      *responseCache
}

func New(host, token string) *Client {
	c := &Client{
		host:  host,
		token: token,
		http: &http.Client{
			Timeout: requestTimeout,
			Transport: &http.Transport{
				Proxy:               http.ProxyFromEnvironment,
				DialContext:         (&net.Dialer{Timeout: dialTimeout}).DialContext,
				TLSHandshakeTimeout: tlsTimeout,
				ForceAttemptHTTP2:   true,
			},
		},
		cache: newResponseCache(maxCacheEntries),
	}
	if host == "" || host == DotComHost {
		c.restURL, c.graphqlURL = dotComRESTURL, dotComGraphQLURL
	} else {
		c.restURL = fmt.Sprintf(enterpriseRESTFmt, host)
		c.graphqlURL = fmt.Sprintf(enterpriseGraphQLFmt, host)
	}
	return c
}

func (c *Client) SetBaseURL(base string) {
	base = strings.TrimRight(base, "/")
	c.restURL = base
	c.graphqlURL = base + graphQLSuffix
}

func (c *Client) resolve(target string) string {
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		return target
	}
	return c.restURL + target
}

func (c *Client) do(ctx context.Context, method, target string, payload, out any) (http.Header, error) {
	full := c.resolve(target)
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("github: encode request: %w", err)
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, full, body)
	if err != nil {
		return nil, fmt.Errorf("github: build request: %w", err)
	}
	req.Header.Set("Accept", acceptJSON)
	req.Header.Set("X-GitHub-Api-Version", apiVersion)
	req.Header.Set("User-Agent", userAgent)
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	cached, hasCached := cacheEntry{}, false
	if method == http.MethodGet {
		if cached, hasCached = c.cache.get(full); hasCached {
			req.Header.Set("If-None-Match", cached.etag)
		}
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github: %s %s: %w", method, full, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("github: read response of %s %s: %w", method, full, err)
	}
	if len(data) > maxResponseBytes {
		return nil, fmt.Errorf("github: response of %s %s exceeds %d bytes", method, full, maxResponseBytes)
	}

	header := resp.Header
	switch {
	case resp.StatusCode == http.StatusNotModified && hasCached:
		data, header = cached.body, cached.header
	case resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices:
		return nil, newResponseError(method, full, resp, data)
	case method == http.MethodGet && resp.Header.Get("ETag") != "":
		c.cache.put(cacheEntry{key: full, etag: resp.Header.Get("ETag"), body: data, header: resp.Header.Clone()})
	}

	if out == nil {
		return header, nil
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("github: %s %s: empty response body", method, full)
	}
	if err := json.Unmarshal(data, out); err != nil {
		return nil, fmt.Errorf("github: decode response of %s %s: %w", method, full, err)
	}
	return header, nil
}

func newResponseError(method, target string, resp *http.Response, data []byte) error {
	var body struct {
		Message string `json:"message"`
	}
	_ = json.Unmarshal(data, &body)
	message := body.Message
	if message == "" {
		message = http.StatusText(resp.StatusCode)
	}
	if reset, ok := rateLimitReset(resp); ok {
		return &RateLimitError{Status: resp.StatusCode, Message: message, Reset: reset}
	}
	return &APIError{Status: resp.StatusCode, Message: message, Method: method, URL: target}
}

func rateLimitReset(resp *http.Response) (time.Time, bool) {
	if resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusTooManyRequests {
		return time.Time{}, false
	}
	retryAfter := resp.Header.Get("Retry-After")
	if resp.Header.Get("X-RateLimit-Remaining") != "0" && retryAfter == "" {
		return time.Time{}, false
	}
	if unix, err := strconv.ParseInt(resp.Header.Get("X-RateLimit-Reset"), 10, 64); err == nil {
		return time.Unix(unix, 0), true
	}
	if seconds, err := strconv.Atoi(retryAfter); err == nil {
		return time.Now().Add(time.Duration(seconds) * time.Second), true
	}
	return time.Time{}, true
}

func (c *Client) graphql(ctx context.Context, query string, variables map[string]any, out any) error {
	var envelope struct {
		Data   json.RawMessage  `json:"data"`
		Errors []GraphQLMessage `json:"errors"`
	}
	payload := map[string]any{"query": query, "variables": variables}
	if _, err := c.do(ctx, http.MethodPost, c.graphqlURL, payload, &envelope); err != nil {
		return err
	}
	if len(envelope.Errors) > 0 {
		for _, m := range envelope.Errors {
			if m.Type == graphQLErrRateLimit {
				return &RateLimitError{Status: http.StatusOK, Message: m.Message}
			}
		}
		return &GraphQLError{Errors: envelope.Errors}
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return errors.New("github: graphql: response has no data")
	}
	if err := json.Unmarshal(envelope.Data, out); err != nil {
		return fmt.Errorf("github: decode graphql data: %w", err)
	}
	return nil
}

func getPages[P any](ctx context.Context, c *Client, path string, each func(P)) error {
	base, err := url.Parse(c.restURL)
	if err != nil {
		return fmt.Errorf("github: invalid base url: %w", err)
	}
	target := path
	for range maxPages {
		var page P
		header, err := c.do(ctx, http.MethodGet, target, nil, &page)
		if err != nil {
			return err
		}
		each(page)
		next := nextLink(header)
		if next == "" {
			return nil
		}
		parsed, err := url.Parse(next)
		if err != nil || parsed.Host != base.Host {
			return fmt.Errorf("github: refusing pagination link %q", next)
		}
		target = next
	}
	return fmt.Errorf("github: more than %d pages for %s", maxPages, path)
}

func listAll[T any](ctx context.Context, c *Client, path string) ([]T, error) {
	var all []T
	err := getPages(ctx, c, path, func(page []T) { all = append(all, page...) })
	return all, err
}

func nextLink(header http.Header) string {
	for _, value := range header.Values("Link") {
		for _, part := range strings.Split(value, ",") {
			target, params, found := strings.Cut(part, ";")
			if !found || !strings.Contains(params, linkRelNext) {
				continue
			}
			target = strings.TrimSpace(target)
			if strings.HasPrefix(target, "<") && strings.HasSuffix(target, ">") {
				return target[1 : len(target)-1]
			}
		}
	}
	return ""
}

func repoURL(r Remote) (string, error) {
	if r.Owner == "" || r.Name == "" {
		return "", errors.New("github: repository owner and name are required")
	}
	return "/repos/" + url.PathEscape(r.Owner) + "/" + url.PathEscape(r.Name), nil
}

func checkNumber(n int) error {
	if n <= 0 {
		return fmt.Errorf("github: invalid pull request number %d", n)
	}
	return nil
}
