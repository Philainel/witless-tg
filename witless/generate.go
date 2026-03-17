package witless

import (
	"math/rand"
	"strings"

	"code.philainel.pw/philainel/witless-tg/core"
	"github.com/lib/pq"
)

func (wt *Witless) Generate(id int64) (string, error) {
	query := `
		SELECT next, count FROM links WHERE token = $1 AND chat = $2
	`
	var current int64 = 1
	tokens := make([]int64, 0, 10)
	for {
		rows, err := wt.db.GetDB().Query(query, current, id)
		if err != nil {
			return "", err
		}
		defer rows.Close()
		nexts := make([]*core.NextCountPair, 0, 10)
		for rows.Next() {
			next := &core.NextCountPair{}
			err := rows.Scan(&next.Next, &next.Count)
			if err != nil {
				return "", err
			}
			nexts = append(nexts, next)
		}
		total := 0;
		for i := range nexts {
			total += nexts[i].Count
		}
		random := rand.Intn(total)
		sum := 0
		var next int64 = 0
		for i := range nexts {
			sum += nexts[i].Count
			if random <= sum {
				next = nexts[i].Next
				break
			}
		}
		if next == 2 {
			break
		}
		tokens = append(tokens, next)
		current = next
	}
	query = `
		SELECT id, word FROM token WHERE id = ANY($1)
	`
	rows, err := wt.db.GetDB().Query(query, pq.Array(tokens))
	if err != nil {
		return "", err
	}
	defer rows.Close()
	tokenToWord := make(map[int64]string)
	for rows.Next() {
		var id int64
		var word string
		if err := rows.Scan(&id, &word); err != nil {
			return "", err
		}
		tokenToWord[id]=word
	}
	words := make([]string, len(tokens));
	for i := range tokens {
		words[i] = tokenToWord[tokens[i]]
	}
	return strings.Join(words, " "), nil
}

