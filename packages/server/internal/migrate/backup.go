//nolint:revive // exported
package migrate

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// BackupManager handles physical database backups.
type BackupManager struct {
	DatabasePath string
	BackupDir    string
	Retain       int
}

// Create makes a copy of the database and its companion WAL/SHM files.
func (b *BackupManager) Create(ctx context.Context, migrationID string, now time.Time) (string, error) {
	if b == nil || b.DatabasePath == "" || b.BackupDir == "" {
		return "", fmt.Errorf("backup manager not configured")
	}

	if err := os.MkdirAll(b.BackupDir, 0o750); err != nil {
		return "", fmt.Errorf("create backup dir: %w", err)
	}

	dirName := fmt.Sprintf("%s-%s", now.UTC().Format("20060102T150405Z"), migrationID)
	targetDir := filepath.Join(b.BackupDir, dirName)
	if err := os.MkdirAll(targetDir, 0o750); err != nil {
		return "", fmt.Errorf("create backup subdir: %w", err)
	}

	files := []string{b.DatabasePath, walPath(b.DatabasePath), shmPath(b.DatabasePath)}
	for _, src := range files {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if _, err := os.Stat(src); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", fmt.Errorf("stat %s: %w", src, err)
		}

		dst := filepath.Join(targetDir, filepath.Base(src))
		if err := copyFile(src, dst); err != nil {
			return "", fmt.Errorf("copy %s: %w", src, err)
		}
	}

	return targetDir, nil
}

// Trim enforces backup retention limits.
func (b *BackupManager) Trim() error {
	if b == nil || b.BackupDir == "" || b.Retain <= 0 {
		return nil
	}

	entries, err := os.ReadDir(b.BackupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read backup dir: %w", err)
	}

	type dirInfo struct {
		name string
		mod  time.Time
	}

	var dirs []dirInfo
	for _, entry := range entries {
		if !entry.IsDir() || !strings.Contains(entry.Name(), "-") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		dirs = append(dirs, dirInfo{name: entry.Name(), mod: info.ModTime()})
	}

	if len(dirs) <= b.Retain {
		return nil
	}

	sort.Slice(dirs, func(i, j int) bool { return dirs[i].mod.After(dirs[j].mod) })
	for _, d := range dirs[b.Retain:] {
		_ = os.RemoveAll(filepath.Join(b.BackupDir, d.name))
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(filepath.Clean(src))
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	out, err := os.Create(filepath.Clean(dst))
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}

func walPath(databasePath string) string {
	return databasePath + "-wal"
}

func shmPath(databasePath string) string {
	return databasePath + "-shm"
}
