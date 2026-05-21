package gracedb

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDB_Backup(t *testing.T) {
	db := testDB(t)

	if _, err := db.CreateCollection("backup_test"); err != nil {
		t.Fatalf("create: %v", err)
	}

	vec := []float32{0.1, 0.2, 0.3, 0.4}
	if _, err := db.Upsert("backup_test", "d1", vec, "backup me", nil, nil); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	backupPath := filepath.Join(t.TempDir(), "gracedb.backup")
	err := db.Backup(backupPath)
	if err != nil {
		t.Fatalf("backup: %v", err)
	}

	info, err := os.Stat(backupPath)
	if err != nil {
		t.Fatalf("stat backup: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("expected non-empty backup file")
	}
}

func TestDB_BackupAndRestore(t *testing.T) {
	db := testDB(t)

	if _, err := db.CreateCollection("restore_test"); err != nil {
		t.Fatalf("create: %v", err)
	}

	vec := []float32{0.1, 0.2, 0.3, 0.4}
	if _, err := db.Upsert("restore_test", "d1", vec, "restore me", nil, nil); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	backupPath := filepath.Join(t.TempDir(), "gracedb.backup")
	if err := db.Backup(backupPath); err != nil {
		t.Fatalf("backup: %v", err)
	}

	// Create a new DB and restore into it.
	restoreDir := t.TempDir()
	db2 := testDB(t, WithPath(restoreDir))

	// Restore backup data into the new DB.
	err := db2.Restore(backupPath)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}

	// Verify the collection exists.
	colls, err := db2.ListCollections()
	if err != nil {
		t.Fatalf("list collections: %v", err)
	}
	found := false
	for _, c := range colls {
		if c.Name == "restore_test" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected restore_test collection after restore")
	}
}

func TestDB_Backup_InvalidPath(t *testing.T) {
	db := testDB(t)

	err := db.Backup("/nonexistent/dir/backup.db")
	if err == nil {
		t.Fatal("expected error for invalid backup path")
	}
}
