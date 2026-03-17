package db

import "database/sql"

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

