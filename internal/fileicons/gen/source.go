package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"
)

const (
	themeRepo     = "material-extensions/vscode-material-icon-theme"
	themeCommit   = "3bf5135d7cb7c41ffcd1c60c5f6afdb6c6e6cc41"
	archiveURL    = "https://codeload.github.com/" + themeRepo + "/tar.gz/" + themeCommit
	fetchTimeout  = 2 * time.Minute
	maxArchive    = 64 << 20
	maxSourceFile = 1 << 20

	licensePath       = "LICENSE"
	fileIconsPath     = "src/core/icons/fileIcons.ts"
	folderIconsPath   = "src/core/icons/folderIcons.ts"
	languageIconsPath = "src/core/icons/languageIcons.ts"
	fileGenPath       = "src/core/generator/fileGenerator.ts"
	folderGenPath     = "src/core/generator/folderGenerator.ts"
	openFolderPath    = "src/scripts/svg/generateOpenFolderIcons.ts"
	defaultCfgPath    = "src/core/generator/config/defaultConfig.ts"
	iconsDir          = "icons/"
	svgSuffix         = ".svg"
)

var wantedSources = map[string]bool{
	licensePath:       true,
	fileIconsPath:     true,
	folderIconsPath:   true,
	languageIconsPath: true,
	fileGenPath:       true,
	folderGenPath:     true,
	openFolderPath:    true,
	defaultCfgPath:    true,
}

func wanted(name string) bool {
	if wantedSources[name] {
		return true
	}
	return strings.HasSuffix(name, svgSuffix) && path.Dir(name)+"/" == iconsDir
}

func download(ctx context.Context) (map[string][]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, archiveURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", archiveURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: status %s", archiveURL, resp.Status)
	}
	return extract(io.LimitReader(resp.Body, maxArchive))
}

func extract(r io.Reader) (map[string][]byte, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("open archive: %w", err)
	}
	defer gz.Close()
	files := map[string][]byte{}
	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read archive: %w", err)
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		_, name, ok := strings.Cut(header.Name, "/")
		if !ok || !wanted(name) {
			continue
		}
		data, err := io.ReadAll(io.LimitReader(tr, maxSourceFile+1))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}
		if len(data) > maxSourceFile {
			return nil, fmt.Errorf("%s is larger than %d bytes", name, maxSourceFile)
		}
		files[name] = data
	}
	for name := range wantedSources {
		if _, ok := files[name]; !ok {
			return nil, fmt.Errorf("archive has no %s", name)
		}
	}
	return files, nil
}
