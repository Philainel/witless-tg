package db

import (
	"database/sql"
	"log"

	"code.philainel.pw/philainel/witless-tg/core"
)

func (db *DB) SaveLinksFromTokenPairs(pairs []*core.TokenPair, id int64) {
	query := `
		INSERT INTO links (token, chat, next, count)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (token, chat, next) DO UPDATE SET count = links.count + 1;
	`
	for _, p := range pairs {
		_, err := db.db.Exec(query, p.Current, id, p.Next, 1);
		if err != nil {
			log.Printf("linking error: %s", err.Error())
			continue
		}
	}
}

type LinkIterator struct {
	rows *sql.Rows
	next core.NextCountPair
	err error
}

func (db *DB) ReadNextAvailableTokens(id int64, current int64) (*LinkIterator, error) {
	query := `
		SELECT next, count FROM links WHERE token = $1 AND chat = $2
	`
	rows, err := db.db.Query(query, current, id)
	if err != nil { return nil, err }
	return &LinkIterator{ rows: rows }, nil
}

func (lit *LinkIterator) Close() {
	lit.rows.Close()
}

func (lit *LinkIterator) Err() error {
	return lit.err
}

func (lit *LinkIterator) NextCountPair() core.NextCountPair {
	return lit.next
}

func (lit *LinkIterator) Next() bool {
	if !lit.rows.Next() {
		return false
	}
	lit.err = lit.rows.Scan(&lit.next.Next, &lit.next.Count)
	return lit.err == nil
}

func (db *DB) ResetLinksForChat(chat int64) error {
	query := `
		DELETE FROM links WHERE chat = $1
	`
	_, err := db.db.Exec(query, chat)
	return err
}

