package jsonc

import (
	"bytes"
	"errors"
	"os"
)

func ReadFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	data = Standard(data)
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, nil
	}
	return data, nil
}
