// Package persistence contains small durable storage helpers shared by domain stores.
package persistence

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/Laisky/errors/v2"
)

const fileMode = 0o600

// LoadJSON receives a path and destination pointer and decodes JSON when the file exists.
func LoadJSON(path string, dst any) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return errors.Wrap(err, "open json store")
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(dst); err != nil {
		return errors.Wrap(err, "decode json store")
	}

	return nil
}

// SaveJSONAtomic receives a path and value and writes JSON with an atomic rename.
func SaveJSONAtomic(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return errors.Wrap(err, "create json store directory")
	}

	temp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return errors.Wrap(err, "create json store temp file")
	}
	tempName := temp.Name()
	removeTemp := true
	defer func() {
		if removeTemp {
			_ = os.Remove(tempName)
		}
	}()

	if err := temp.Chmod(fileMode); err != nil {
		_ = temp.Close()
		return errors.Wrap(err, "set json store temp mode")
	}
	encoder := json.NewEncoder(temp)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		_ = temp.Close()
		return errors.Wrap(err, "encode json store")
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return errors.Wrap(err, "sync json store")
	}
	if err := temp.Close(); err != nil {
		return errors.Wrap(err, "close json store temp file")
	}
	if err := os.Rename(tempName, path); err != nil {
		return errors.Wrap(err, "replace json store")
	}
	removeTemp = false

	return nil
}
