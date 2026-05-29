package db

import (
	"database/sql"


	"github.com/lib/pq"
)

func (db *DB) GetTokensByWords(words []string) ([]int64, error) {
	query := `
		SELECT id FROM token WHERE word = $1
	`
	insert := `
		INSERT INTO token (word) VALUES ($1) RETURNING id
	`
	var id int64
	result := make([]int64, 0, len(words))
	for _, w := range words {
		err := db.db.QueryRow(query, w).Scan(&id); 
		if err == sql.ErrNoRows {
			err = db.db.QueryRow(insert, w).Scan(&id);
		}
		if err != nil {
			return nil, err
		}
		result = append(result, id);
	}
	return result, nil
}

func (db *DB) TranslateTokensToWords(tokens []int64) ([]string, error) {
	query := `
		SELECT word
		FROM token
		WHERE id = ANY($1)
		ORDER BY array_position($1, id)
	`
	rows, err := db.db.Query(query, pq.Array(tokens))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	words := make([]string, 0, len(tokens));
	for rows.Next() {
		var word string
		if err := rows.Scan(&word); err != nil {
			return nil, err
		}
		words = append(words, word)
	}
	return words, nil
}

