package batch

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateBatchID(t *testing.T) {
	tests := []struct {
		id    string
		valid bool
	}{
		{"batch-20260628T120000Z", true},
		{"", false},
		{"not-batch", false},
		{"batch-evil/../etc", false},
		{"batch-evil\\foo", false},
	}
	for _, tc := range tests {
		err := ValidateBatchID(tc.id)
		if tc.valid && err != nil {
			t.Errorf("ValidateBatchID(%q) = %v, want nil", tc.id, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("ValidateBatchID(%q) = nil, want error", tc.id)
		}
	}
}

func TestResolveBatchDirRejectsEscape(t *testing.T) {
	root := t.TempDir()
	_, err := ResolveBatchDir(root, "batch-../../../etc")
	if err != ErrInvalidBatchID {
		t.Fatalf("err = %v, want ErrInvalidBatchID", err)
	}
}

func TestDeleteBatch(t *testing.T) {
	root := t.TempDir()
	batchID := "batch-test-delete"
	batchDir := filepath.Join(root, batchID)
	if err := os.MkdirAll(filepath.Join(batchDir, "jobs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(batchDir, "batch-summary.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := DeleteBatch(root, batchID); err != nil {
		t.Fatalf("DeleteBatch: %v", err)
	}
	if _, err := os.Stat(batchDir); !os.IsNotExist(err) {
		t.Fatalf("batch dir still exists after delete")
	}
}

func TestDeleteBatchNotFound(t *testing.T) {
	root := t.TempDir()
	err := DeleteBatch(root, "batch-missing")
	if err != ErrBatchNotFound {
		t.Fatalf("err = %v, want ErrBatchNotFound", err)
	}
}
