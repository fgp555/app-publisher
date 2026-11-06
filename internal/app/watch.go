package app

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type watchedFileState struct {
	size        int64
	modifiedAt  int64
	permissions fs.FileMode
}

func watchProjectFiles(ctx context.Context, root string) <-chan error {
	events := make(chan error, 1)
	go func() {
		defer close(events)
		previous, err := snapshotProjectFiles(root)
		if err != nil {
			events <- err
			return
		}

		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		var changedAt time.Time
		pending := false

		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				current, err := snapshotProjectFiles(root)
				if err != nil {
					select {
					case events <- err:
					case <-ctx.Done():
					}
					return
				}
				if !sameFileSnapshot(previous, current) {
					previous = current
					changedAt = now
					pending = true
				}
				if pending && now.Sub(changedAt) >= 750*time.Millisecond {
					select {
					case events <- nil:
						pending = false
					case <-ctx.Done():
						return
					default:
					}
				}
			}
		}
	}()
	return events
}

func snapshotProjectFiles(root string) (map[string]watchedFileState, error) {
	snapshot := make(map[string]watchedFileState)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) {
				return nil
			}
			return walkErr
		}
		if entry.IsDir() {
			if path != root && isGeneratedOrDependencyDir(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		relativePath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		snapshot[relativePath] = watchedFileState{
			size:        info.Size(),
			modifiedAt:  info.ModTime().UnixNano(),
			permissions: info.Mode().Perm(),
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return snapshot, nil
}

func isGeneratedOrDependencyDir(name string) bool {
	switch strings.ToLower(name) {
	case ".git", ".gradle", ".idea", ".kotlin", ".cxx", ".expo", "build", "node_modules", "out":
		return true
	default:
		return false
	}
}

func sameFileSnapshot(left, right map[string]watchedFileState) bool {
	if len(left) != len(right) {
		return false
	}
	for path, state := range left {
		if right[path] != state {
			return false
		}
	}
	return true
}
