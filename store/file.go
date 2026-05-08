package store

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/wdspkr/ausfallplan-notifier/ausfallplan"
)

// FileStore persists a Snapshot to a single JSON file.
// On Load, if the file is missing, returns an empty Snapshot and nil error
// (this is the "first run" case — diffing against empty yields all current
// entries as additions, which is exactly what we want).
type FileStore struct {
	Path string
}

// NewFileStore returns a FileStore that uses path as its backing file.
func NewFileStore(path string) *FileStore {
	return &FileStore{Path: path}
}

// Load reads and JSON-unmarshals a Snapshot from f.Path.
// If the file does not exist, it returns Snapshot{}, nil.
func (f *FileStore) Load(_ context.Context) (ausfallplan.Snapshot, error) {
	data, err := os.ReadFile(f.Path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return ausfallplan.Snapshot{}, nil
		}
		return ausfallplan.Snapshot{}, err
	}

	var snap ausfallplan.Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return ausfallplan.Snapshot{}, err
	}
	return snap, nil
}

// Save sorts the snapshot entries canonically, then writes it as indented
// JSON to f.Path atomically (via a temp file + rename).
// File mode is 0644.
func (f *FileStore) Save(_ context.Context, snap ausfallplan.Snapshot) error {
	// Sort Entries by Day, Hour, Class, Information for stable output.
	entries := make([]ausfallplan.Entry, len(snap.Entries))
	copy(entries, snap.Entries)
	sort.Slice(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if !a.Day.Equal(b.Day) {
			return a.Day.Before(b.Day)
		}
		if a.Hour != b.Hour {
			return a.Hour < b.Hour
		}
		if a.Class != b.Class {
			return a.Class < b.Class
		}
		return a.Information < b.Information
	})

	// Sort Infos by Text.
	infos := make([]ausfallplan.Info, len(snap.Infos))
	copy(infos, snap.Infos)
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Text < infos[j].Text
	})

	canonical := ausfallplan.Snapshot{Entries: entries, Infos: infos}

	data, err := json.MarshalIndent(canonical, "", "  ")
	if err != nil {
		return err
	}
	// Append newline so the file is well-formed text.
	data = append(data, '\n')

	dir := filepath.Dir(f.Path)
	tmp, err := os.CreateTemp(dir, ".state-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	// Clean up temp file on any error path.
	defer func() {
		if tmp != nil {
			tmp.Close()
			os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	tmp = nil // prevent the defer from double-closing

	if err := os.Chmod(tmpName, 0644); err != nil {
		os.Remove(tmpName)
		return err
	}

	return os.Rename(tmpName, f.Path)
}
