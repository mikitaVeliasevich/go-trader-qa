package batch

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

var (
	ErrInvalidBatchID = errors.New("invalid batch id")
	ErrBatchNotFound  = errors.New("batch not found")
)

// ValidateBatchID rejects empty, malformed, or path-traversal batch IDs.
func ValidateBatchID(id string) error {
	if id == "" {
		return ErrInvalidBatchID
	}
	if !strings.HasPrefix(id, "batch-") {
		return ErrInvalidBatchID
	}
	if strings.Contains(id, "/") || strings.Contains(id, "\\") || strings.Contains(id, "..") {
		return ErrInvalidBatchID
	}
	return nil
}

// ResolveBatchDir returns the absolute batch directory under artifactsDir.
func ResolveBatchDir(artifactsDir, batchID string) (string, error) {
	if err := ValidateBatchID(batchID); err != nil {
		return "", err
	}
	root := filepath.Clean(artifactsDir)
	batchDir := filepath.Clean(filepath.Join(root, batchID))
	if batchDir != root && !strings.HasPrefix(batchDir, root+string(filepath.Separator)) {
		return "", ErrInvalidBatchID
	}
	return batchDir, nil
}

// DeleteBatch removes the entire batch directory tree.
func DeleteBatch(artifactsDir, batchID string) error {
	batchDir, err := ResolveBatchDir(artifactsDir, batchID)
	if err != nil {
		return err
	}
	info, err := os.Stat(batchDir)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrBatchNotFound
		}
		return err
	}
	if !info.IsDir() {
		return ErrBatchNotFound
	}
	return os.RemoveAll(batchDir)
}
