package gracedb

import (
	"bytes"
	"fmt"
	"os"
)

// Backup creates a full database backup to the given path.
func (db *DB) Backup(path string) error {
	// Flush memtable before backup.
	if err := db.store_.DB().Sync(); err != nil {
		return fmt.Errorf("flush before backup: %w", err)
	}

	// Create the backup file.
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create backup file: %w", err)
	}
	defer f.Close()

	// Use Badger native backup API (returns writer, error).
	_, err = db.store_.DB().Backup(f, 0)
	if err != nil {
		return fmt.Errorf("badger backup: %w", err)
	}
	return nil
}

// Restore restores the database from a backup at the given path.
// The backup data is loaded into the existing database.
func (db *DB) Restore(backupPath string) error {
	// Read the backup file.
	data, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("read backup file: %w", err)
	}

	// Load backup data. Badger Load requires a reader and number of workers.
	r := bytes.NewReader(data)
	if err := db.store_.DB().Load(r, 1); err != nil {
		return fmt.Errorf("badger restore: %w", err)
	}

	// Rebuild all collection indexes after restore.
	colls, err := db.store_.ListCollections()
	if err != nil {
		return fmt.Errorf("list collections after restore: %w", err)
	}
	for _, coll := range colls {
		if err := db.store_.LoadIndex(coll.Name); err != nil {
			return fmt.Errorf("load index %s after restore: %w", coll.Name, err)
		}
	}
	return nil
}
