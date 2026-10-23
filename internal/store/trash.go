package store

import (
	"os"
	"path/filepath"
)

// trashDir returns the per-project trash folder.
func trashDir(loc Location) string {
	return filepath.Join(loc.ProjectDir, "trash")
}

// Trash moves a file into the project's trash folder.
// The relative path is preserved so it can be restored.
func Trash(loc Location, rel string) (string, error) {
	src := filepath.Join(loc.Dir, filepath.FromSlash(rel))
	info, err := os.Stat(src)
	if err != nil {
		return "", err
	}
	dst := filepath.Join(trashDir(loc), filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return "", err
	}
	if err := os.Rename(src, dst); err != nil {
		return "", err
	}
	// Preserve mode best-effort.
	_ = os.Chmod(dst, info.Mode().Perm())
	return dst, nil
}

// Restore moves a file from the trash folder back into the env folder.
func Restore(loc Location, rel string) error {
	src := filepath.Join(trashDir(loc), filepath.FromSlash(rel))
	dst := filepath.Join(loc.Dir, filepath.FromSlash(rel))
	if _, err := os.Stat(src); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	return os.Rename(src, dst)
}

// ListTrash returns the relative paths of trashed env files.
func ListTrash(loc Location) ([]string, error) {
	var files []string
	root := trashDir(loc)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return filepath.SkipDir
			}
			return err
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if os.IsNotExist(err) {
		return nil, nil
	}
	return files, err
}

// EmptyTrash permanently removes all trashed files.
func EmptyTrash(loc Location) (int, error) {
	root := trashDir(loc)
	files, err := ListTrash(loc)
	if err != nil {
		return 0, err
	}
	count := len(files)
	if err := os.RemoveAll(root); err != nil {
		return 0, err
	}
	return count, nil
}
