package db

import (
	"fmt"
	"database/sql"
)

type DB struct {
	db *sql.DB;
}

func NewDB(user, password, host, dbname string) (*DB, error) {
	db, err := sql.Open(
		"postgres", 
		fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable", user, password, host, dbname),
	)
	return &DB{ db: db }, err
}

func (d *DB) Close() {
	d.db.Close()
}

