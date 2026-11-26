package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	sourceRepo   = "codebling/vs-code-default-keybindings"
	sourceCommit = "d3bf0f60c263ada63bb56d19f86cc1c9ea696ff4"
	rawBaseURL   = "https://raw.githubusercontent.com/" + sourceRepo + "/" + sourceCommit + "/"
	fetchTimeout = 2 * time.Minute
	maxDumpBytes = 8 << 20
)

type platform struct {
	name     string
	variable string
	file     string
}

var platforms = []platform{
	{name: "macOS", variable: "MacOS", file: "macos.keybindings.json"},
	{name: "Windows", variable: "Windows", file: "windows.keybindings.json"},
}

func download(ctx context.Context) ([][]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()
	out := make([][]byte, len(platforms))
	for i, p := range platforms {
		data, err := fetch(ctx, rawBaseURL+p.file)
		if err != nil {
			return nil, err
		}
		out[i] = data
	}
	return out, nil
}

func fetch(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: status %s", url, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxDumpBytes+1))
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", url, err)
	}
	if len(data) > maxDumpBytes {
		return nil, fmt.Errorf("download %s: larger than %d bytes", url, maxDumpBytes)
	}
	return data, nil
}
