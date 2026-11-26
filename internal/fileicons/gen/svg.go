package main

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"regexp"
)

const openFolderViewBox = `viewBox="0 0 16 16"`

const (
	defaultFileName   = "file"
	defaultFolderName = "folder"
	rootFolderName    = "folder-root"
	openSuffix        = "-open"
	lightSuffix       = "_light"
	svgTemplate       = `<svg viewBox="0 0 16 16" xmlns="http://www.w3.org/2000/svg">%s</svg>`
	pathTemplate      = `<path d="%s" fill="%s"/>`
	rootOuterTemplate = `<circle cx="8" cy="8" r="6" fill="none" stroke="%s" stroke-width="2"/>`
	rootInnerTemplate = `<circle cx="8" cy="8" r="3" fill="%s"/>`
)

var (
	commentPattern    = regexp.MustCompile(`(?s)<!--.*?-->`)
	newlinePattern    = regexp.MustCompile(`\s*[\r\n]+\s*`)
	betweenTags       = regexp.MustCompile(`>\s+<`)
	folderPathPattern = regexp.MustCompile(`<path\b[^>]*\bid="folder"[^>]*>`)
	pathDataPattern   = regexp.MustCompile(`\bd="[^"]*"`)
)

func minify(svg []byte) []byte {
	svg = commentPattern.ReplaceAll(svg, nil)
	svg = newlinePattern.ReplaceAll(svg, []byte(" "))
	svg = betweenTags.ReplaceAll(svg, []byte("><"))
	return bytes.TrimSpace(svg)
}

func validateSVG(svg []byte) error {
	dec := xml.NewDecoder(bytes.NewReader(svg))
	root := ""
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("invalid XML: %w", err)
		}
		if start, ok := tok.(xml.StartElement); ok && root == "" {
			root = start.Name.Local
		}
	}
	if root != "svg" {
		return fmt.Errorf("root element is %q, want svg", root)
	}
	return nil
}

func openFolder(closed []byte, openPath string) ([]byte, error) {
	if !bytes.Contains(closed, []byte(openFolderViewBox)) {
		return nil, fmt.Errorf("viewBox is not 0 0 16 16")
	}
	loc := folderPathPattern.FindIndex(closed)
	if loc == nil {
		return nil, fmt.Errorf("no path with id=\"folder\"")
	}
	tag := closed[loc[0]:loc[1]]
	if !pathDataPattern.Match(tag) {
		return nil, fmt.Errorf("folder path has no d attribute")
	}
	replaced := pathDataPattern.ReplaceAllLiteral(tag, []byte(`d="`+openPath+`"`))
	out := make([]byte, 0, len(closed)+len(openPath))
	out = append(out, closed[:loc[0]]...)
	out = append(out, replaced...)
	out = append(out, closed[loc[1]:]...)
	return out, nil
}

type defaultShapes struct {
	filePath   string
	folderPath string
	openPath   string
	color      string
}

var (
	filePathPattern   = regexp.MustCompile(`const fileIcon =\s*'([^']+)'`)
	folderPathSource  = regexp.MustCompile(`const folderIcon =\s*'([^']+)'`)
	openPathSource    = regexp.MustCompile(`OPEN_FOLDER_PATH =\s*'([^']+)'`)
	defaultColorToken = regexp.MustCompile(`const defaultColor = '(#[0-9a-fA-F]{6})'`)
)

func extractShapes(sources map[string][]byte) (defaultShapes, error) {
	find := func(re *regexp.Regexp, file string) (string, error) {
		m := re.FindSubmatch(sources[file])
		if m == nil {
			return "", fmt.Errorf("%s: pattern %s not found", file, re)
		}
		return string(m[1]), nil
	}
	var s defaultShapes
	var err error
	if s.filePath, err = find(filePathPattern, fileGenPath); err != nil {
		return s, err
	}
	if s.folderPath, err = find(folderPathSource, folderGenPath); err != nil {
		return s, err
	}
	if s.openPath, err = find(openPathSource, openFolderPath); err != nil {
		return s, err
	}
	if s.color, err = find(defaultColorToken, defaultCfgPath); err != nil {
		return s, err
	}
	return s, nil
}

func (s defaultShapes) svgs() map[string][]byte {
	wrap := func(body string) []byte { return []byte(fmt.Sprintf(svgTemplate, body)) }
	return map[string][]byte{
		defaultFileName:                wrap(fmt.Sprintf(pathTemplate, s.filePath, s.color)),
		defaultFolderName:              wrap(fmt.Sprintf(pathTemplate, s.folderPath, s.color)),
		defaultFolderName + openSuffix: wrap(fmt.Sprintf(pathTemplate, s.openPath, s.color)),
		rootFolderName:                 wrap(fmt.Sprintf(rootOuterTemplate, s.color) + fmt.Sprintf(rootInnerTemplate, s.color)),
		rootFolderName + openSuffix:    wrap(fmt.Sprintf(rootOuterTemplate, s.color)),
	}
}
