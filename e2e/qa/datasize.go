package qa

import (
	"io/fs"
	"path/filepath"
	"strings"
)

const (
	DataConfig  = "config"
	DataHistory = "history"
	DataBackups = "backups"
	DataShelf   = "shelf"
	DataLogs    = "logs"
	DataOther   = "other"
	DataTemp    = "temp"
	dataDir     = "data"
	logsDir     = "logs"
)

func DataCategory(rel string) string {
	parts := strings.Split(filepath.ToSlash(rel), "/")
	switch {
	case len(parts) == 1:
		return DataConfig
	case parts[0] == logsDir:
		return DataLogs
	case parts[0] == dataDir && len(parts) > 2:
		switch parts[1] {
		case DataHistory, DataBackups, DataShelf:
			return parts[1]
		}
	}
	return DataOther
}

func DataBytes(userData, temp string) map[string]int64 {
	sizes := map[string]int64{}
	walkBytes(userData, func(rel string, size int64) { sizes[DataCategory(rel)] += size })
	walkBytes(temp, func(_ string, size int64) { sizes[DataTemp] += size })
	return sizes
}

func TotalBytes(sizes map[string]int64) int64 {
	var total int64
	for _, size := range sizes {
		total += size
	}
	return total
}

func walkBytes(root string, add func(rel string, size int64)) {
	if root == "" {
		return
	}
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		add(rel, info.Size())
		return nil
	})
}
