package db;

import (
	"log"
	"database/sql"

	core "code.philainel.pw/philainel/witless-tg/core"
)

func SaveLinksFromTokenPairs(db *sql.DB, pairs []*core.TokenPair, id int64) {
	query := `
		INSERT INTO links (token, chat, next, count)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (token, chat, next) DO UPDATE SET count = links.count + 1;
	`
	for _, p := range pairs {
		_, err := db.Exec(query, p.Current, id, p.Next, 1);
		if err != nil {
			log.Printf("linking error: %s", err.Error())
			continue
		}
	}
}
