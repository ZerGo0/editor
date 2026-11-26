package main

import (
	"bufio"
	"context"
	_ "embed"
	"fmt"
	"log"
	"os"
	"strings"
)

//go:embed allowlist.txt
var allowlistText string

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	sources, err := download(context.Background())
	if err != nil {
		return err
	}
	allow, err := parseAllowlist(allowlistText)
	if err != nil {
		return err
	}
	t, err := build(sources, allow)
	if err != nil {
		return err
	}
	if err := writeOutputs(".", t, sources[licensePath]); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "wrote %d icons (%d SVG files, %d bytes), %d file names, %d extensions, %d folder names, %d languages\n",
		len(t.defs), len(t.files), t.embedded, len(t.fileNames), len(t.extensions), len(t.folders), len(t.languages))
	return nil
}

func parseAllowlist(text string) (map[string]bool, error) {
	allow := map[string]bool{}
	sc := bufio.NewScanner(strings.NewReader(text))
	for sc.Scan() {
		name := strings.TrimSpace(sc.Text())
		if name == "" {
			continue
		}
		if allow[name] {
			return nil, fmt.Errorf("allowlist lists %q twice", name)
		}
		allow[name] = true
	}
	return allow, sc.Err()
}
