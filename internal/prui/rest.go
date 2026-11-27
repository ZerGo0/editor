package prui

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
	"sync"
	"time"

	"github.com/ZerGo0/editor/internal/github"
)

const (
	dotComRESTURL        = "https://api.github.com"
	dotComGraphQLURL     = "https://api.github.com/graphql"
	enterpriseRESTFmt    = "https://%s/api/v3"
	enterpriseGraphQLFmt = "https://%s/api/graphql"
	restTimeout          = 30 * time.Second
	restDialTimeout      = 10 * time.Second
	restTLSTimeout       = 10 * time.Second
	restUserAgent        = "editor-prui"
	restAPIVersion       = "2022-11-28"
	acceptJSON           = "application/vnd.github+json"
	acceptRaw            = "application/vnd.github.raw+json"
	acceptText           = "text/plain"
	maxFileBytes         = 4 << 20
	maxJSONBytes         = 16 << 20
	maxLogBytes          = 8 << 20
	maxCachedFiles       = 64
	maxTailRead          = 64 << 20
	rateLimitedType      = "RATE_LIMITED"
)

var ErrFileTooLarge = errors.New("prui: file is too large to show")

type restClient struct {
	base       string
	graphqlURL string
	token      string
	http       *http.Client
	mu         sync.Mutex
	files      map[string][]byte
}

func newRESTClient(host, token string) *restClient {
	r := &restClient{
		base:       dotComRESTURL,
		graphqlURL: dotComGraphQLURL,
		token:      token,
		files:      make(map[string][]byte),
		http: &http.Client{
			Timeout: restTimeout,
			Transport: &http.Transport{
				Proxy:               http.ProxyFromEnvironment,
				DialContext:         (&net.Dialer{Timeout: restDialTimeout}).DialContext,
				TLSHandshakeTimeout: restTLSTimeout,
				ForceAttemptHTTP2:   true,
			},
		},
	}
	if host != "" && host != github.DotComHost {
		r.base = fmt.Sprintf(enterpriseRESTFmt, host)
		r.graphqlURL = fmt.Sprintf(enterpriseGraphQLFmt, host)
	}
	return r
}

type request struct {
	method string
	target string
	accept string
	body   any
	limit  int64
	header map[string]string
	tail   bool
}

type response struct {
	data   []byte
	header http.Header
	status int
}

func (r *restClient) send(ctx context.Context, q request) (response, error) {
	var body io.Reader
	if q.body != nil {
		encoded, err := json.Marshal(q.body)
		if err != nil {
			return response{}, fmt.Errorf("prui: encode request: %w", err)
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, q.method, q.target, body)
	if err != nil {
		return response{}, fmt.Errorf("prui: build request: %w", err)
	}
	accept := q.accept
	if accept == "" {
		accept = acceptJSON
	}
	req.Header.Set("Accept", accept)
	req.Header.Set("X-GitHub-Api-Version", restAPIVersion)
	req.Header.Set("User-Agent", restUserAgent)
	if r.token != "" {
		req.Header.Set("Authorization", "Bearer "+r.token)
	}
	if q.body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range q.header {
		req.Header.Set(k, v)
	}
	resp, err := r.http.Do(req)
	if err != nil {
		return response{}, fmt.Errorf("prui: %s %s: %w", q.method, q.target, err)
	}
	defer resp.Body.Close()
	limit := q.limit
	if limit == 0 {
		limit = maxJSONBytes
	}
	readLimit := limit + 1
	if q.tail {
		readLimit = maxTailRead
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, readLimit))
	if err != nil {
		return response{}, fmt.Errorf("prui: read response of %s %s: %w", q.method, q.target, err)
	}
	out := response{data: data, header: resp.Header, status: resp.StatusCode}
	if resp.StatusCode == http.StatusNotModified {
		return out, nil
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return out, statusError(q, resp, data)
	}
	if int64(len(data)) > limit {
		if q.tail {
			out.data = data[int64(len(data))-limit:]
			return out, nil
		}
		return out, fmt.Errorf("%w: %s %s exceeds %d bytes", ErrFileTooLarge, q.method, q.target, limit)
	}
	return out, nil
}

func statusError(q request, resp *http.Response, data []byte) error {
	var body struct {
		Message string `json:"message"`
	}
	_ = json.Unmarshal(data, &body)
	if body.Message == "" {
		body.Message = http.StatusText(resp.StatusCode)
	}
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		if resp.Header.Get("X-RateLimit-Remaining") == "0" || resp.Header.Get("Retry-After") != "" {
			limit := &github.RateLimitError{Status: resp.StatusCode, Message: body.Message}
			if unix, err := strconv.ParseInt(resp.Header.Get("X-RateLimit-Reset"), 10, 64); err == nil {
				limit.Reset = time.Unix(unix, 0)
			}
			return limit
		}
	}
	return &github.APIError{Status: resp.StatusCode, Message: body.Message, Method: q.method, URL: q.target}
}

func (r *restClient) json(ctx context.Context, method, path string, payload, out any) error {
	res, err := r.send(ctx, request{method: method, target: r.base + path, body: payload})
	if err != nil {
		return err
	}
	if out == nil || len(res.data) == 0 {
		return nil
	}
	if err := json.Unmarshal(res.data, out); err != nil {
		return fmt.Errorf("prui: decode response of %s %s: %w", method, path, err)
	}
	return nil
}

func (r *restClient) gql(ctx context.Context, query string, vars map[string]any, out any) error {
	res, err := r.send(ctx, request{method: http.MethodPost, target: r.graphqlURL, body: map[string]any{"query": query, "variables": vars}})
	if err != nil {
		return err
	}
	var envelope struct {
		Data   json.RawMessage         `json:"data"`
		Errors []github.GraphQLMessage `json:"errors"`
	}
	if err := json.Unmarshal(res.data, &envelope); err != nil {
		return fmt.Errorf("prui: decode graphql response: %w", err)
	}
	for _, m := range envelope.Errors {
		if m.Type == rateLimitedType {
			return &github.RateLimitError{Status: http.StatusOK, Message: m.Message}
		}
	}
	if len(envelope.Errors) > 0 {
		return &github.GraphQLError{Errors: envelope.Errors}
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return errors.New("prui: graphql response has no data")
	}
	if err := json.Unmarshal(envelope.Data, out); err != nil {
		return fmt.Errorf("prui: decode graphql data: %w", err)
	}
	return nil
}

func repoPath(repo github.Remote) (string, error) {
	if repo.Owner == "" || repo.Name == "" {
		return "", errors.New("prui: repository owner and name are required")
	}
	return "/repos/" + url.PathEscape(repo.Owner) + "/" + url.PathEscape(repo.Name), nil
}

func escapePath(path string) string {
	segments := strings.Split(path, "/")
	for i, s := range segments {
		segments[i] = url.PathEscape(s)
	}
	return strings.Join(segments, "/")
}

func (r *restClient) MergeBase(ctx context.Context, repo github.Remote, base, head string) (string, error) {
	root, err := repoPath(repo)
	if err != nil {
		return "", err
	}
	if base == "" || head == "" {
		return "", errors.New("prui: merge base needs base and head")
	}
	var result struct {
		MergeBase *struct {
			SHA string `json:"sha"`
		} `json:"merge_base_commit"`
	}
	if err := r.json(ctx, http.MethodGet, root+"/compare/"+url.PathEscape(base)+"..."+url.PathEscape(head)+"?per_page=1", nil, &result); err != nil {
		return "", err
	}
	if result.MergeBase == nil || result.MergeBase.SHA == "" {
		return "", errors.New("prui: compare response lacks merge base")
	}
	return result.MergeBase.SHA, nil
}

func (r *restClient) FileAt(ctx context.Context, repo github.Remote, ref, path string) ([]byte, error) {
	root, err := repoPath(repo)
	if err != nil {
		return nil, err
	}
	if ref == "" || path == "" {
		return nil, errors.New("prui: file needs ref and path")
	}
	target := r.base + root + "/contents/" + escapePath(path) + "?ref=" + url.QueryEscape(ref)
	r.mu.Lock()
	cached, ok := r.files[target]
	r.mu.Unlock()
	if ok {
		return cached, nil
	}
	res, err := r.send(ctx, request{method: http.MethodGet, target: target, accept: acceptRaw, limit: maxFileBytes})
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	if len(r.files) >= maxCachedFiles {
		clear(r.files)
	}
	r.files[target] = res.data
	r.mu.Unlock()
	return res.data, nil
}
