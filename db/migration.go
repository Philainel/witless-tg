package db

import (
	"fmt"
	"log"
	"os"
	"errors"
	"io/fs"
	"strings"
	"strconv"

	_ "github.com/lib/pq"
)

var (
	migrations []*migration
)

func (d *DB) PerformMigration(version int) error {
	query := `CREATE TABLE IF NOT EXISTS __witless_schema_migration (
		id SERIAL PRIMARY KEY,
		version INT,
		updated_at TIMESTAMPTZ
	)`

	if _, err := d.db.Exec(query); err != nil {
		return fmt.Errorf("failed to create __witless_schema_migration table: %s", err.Error())
	}

	query = `
		INSERT INTO __witless_schema_migration (id, version, updated_at)
		VALUES (1, 0, NOW())
		ON CONFLICT (id) DO NOTHING
	`
	if _, err := d.db.Exec(query); err != nil {
		return fmt.Errorf("failed to ensure version number stored: %s", err.Error())
	}

	query = `SELECT version FROM __witless_schema_migration WHERE id = 1`
	var stored_version int
	err := d.db.QueryRow(query).Scan(&stored_version)
	if err != nil {
		return err
	}
	if stored_version >= version {
		return nil
	}
	log.Printf("PERFORMING MIGRATION from %d to %d...", stored_version, version)
	migrations, err = readMigrations()
	if err != nil {
		return err
	}
	for i := stored_version + 1; i <= version; i++ {
		if err := d.executeMigration(i); err != nil {
			log.Fatalf("Failed to apply migration %d: %s", i, err.Error())
		}
	}
	return nil
}

func (d *DB) executeMigration(version int) error {
	log.Printf("Executing migration %d", version)
	path, err := getMigrationFilePath(version)
	if err != nil {
		return err
	}
	file, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	query := string(file)
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(query); err != nil {
		_ = tx.Rollback()
		return err
	}
	query = `UPDATE __witless_schema_migration SET version = $1, updated_at = NOW() WHERE id = 1`
	if _, err := tx.Exec(query, version); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

type migration struct {
	Version int
	Name string
}

func readMigrations() ([]*migration, error) {
	entries, err := os.ReadDir("./migration")
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) { return nil, nil }
		return nil, err
	}
	result := make([]*migration, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue			
		}
		name := e.Name()
		if !(strings.HasSuffix(name, ".up.sql") || strings.HasSuffix(name, ".down.sql")) {
			continue
		}
		parts := strings.SplitN(name, "_", 2)
		if len(parts) < 2 {
			continue
		}
		ver, err := strconv.Atoi(parts[0])
		if err != nil {
			return nil, err
		}
		m := &migration {
			Version: ver,
			Name: strings.TrimSuffix(parts[1], ".up.sql"),
		}
		result = append(result, m)
	}
	return result, nil
}

func getMigrationFilePath(version int) (string, error) {
	for _, m := range migrations {
		if m.Version == version {
			return fmt.Sprintf("./migration/%.4d_%s.up.sql", m.Version, m.Name), nil
		}
	}
	return "", fmt.Errorf("migration %d not found", version)
}
