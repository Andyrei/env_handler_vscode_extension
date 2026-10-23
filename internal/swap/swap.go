package swap

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"env-handler/internal/fsx"
)

// State describes an active swap.
type State struct {
	Target string `json:"target"` // the env file the project's .env points to
	Backup string `json:"backup"` // the original .env backup
	Mode   string `json:"mode"`   // "symlink" or "copy"
	Hash   string `json:"hash"`   // SHA-256 of the copied content (copy mode only)
}

// Enable creates a swap: .env in the project becomes a symlink (or copy) of target.
// It backs up the existing .env first.
func Enable(root, target, backup string, forceCopy bool) (State, error) {
	targetFull := filepath.Join(root, filepath.FromSlash(target))
	if _, err := os.Stat(targetFull); err != nil {
		return State{}, fmt.Errorf("target env file not found: %s", target)
	}

	envPath := filepath.Join(root, ".env")

	// Back up existing .env if it exists and is not already a managed swap.
	if info, err := os.Lstat(envPath); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			// Already a symlink; remove it to retarget.
			if err := os.Remove(envPath); err != nil {
				return State{}, err
			}
		} else {
			// Move to backup.
			if _, err := os.Stat(filepath.Join(root, backup)); err == nil {
				return State{}, fmt.Errorf("backup already exists: %s (use a different name or restore first)", backup)
			}
			if err := os.Rename(envPath, filepath.Join(root, backup)); err != nil {
				return State{}, err
			}
		}
	}

	// Try symlink first, fall back to copy.
	if !forceCopy {
		relTarget, err := filepath.Rel(filepath.Dir(envPath), targetFull)
		if err == nil {
			if err := os.Symlink(relTarget, envPath); err == nil {
				return State{Target: target, Backup: backup, Mode: "symlink", Hash: ""}, nil
			}
		}
	}

	// Copy fallback.
	data, err := os.ReadFile(targetFull)
	if err != nil {
		return State{}, err
	}
	hash := sha256Hex(data)
	if err := fsx.WriteFileAtomic(envPath, data, 0o644); err != nil {
		return State{}, err
	}
	return State{Target: target, Backup: backup, Mode: "copy", Hash: hash}, nil
}

// Restore removes the swapped .env and restores the backup.
func Restore(root string, state State, force bool) error {
	envPath := filepath.Join(root, ".env")

	// Verify the current .env is still the managed swap.
	info, err := os.Lstat(envPath)
	if err != nil {
		return fmt.Errorf("no .env to restore: %v", err)
	}

	if info.Mode()&os.ModeSymlink != 0 {
		link, err := os.Readlink(envPath)
		if err != nil {
			return err
		}
		targetFull, _ := filepath.EvalSymlinks(filepath.Join(root, state.Target))
		if !strings.Contains(link, filepath.Base(state.Target)) {
			if !force {
				return fmt.Errorf("current .env does not point to the swapped target; use --force to override")
			}
		}
		_ = targetFull
	} else if state.Mode == "copy" {
		data, err := os.ReadFile(envPath)
		if err != nil {
			return err
		}
		if sha256Hex(data) != state.Hash && !force {
			return fmt.Errorf("current .env was modified since swap; use --force to override")
		}
	}

	if err := os.Remove(envPath); err != nil {
		return err
	}

	// Restore backup.
	backupPath := filepath.Join(root, state.Backup)
	if _, err := os.Stat(backupPath); err == nil {
		if err := os.Rename(backupPath, envPath); err != nil {
			return err
		}
	}
	return nil
}

func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
