package witless

import (
	"fmt"
	"math/rand/v2"
	"strings"
)

func (wt *Witless) Generate(id int64) (string, error) {
	var current int64 = 1
	tokens := make([]int64, 0, 10)
	for {
		links, err := wt.db.ReadNextAvailableTokens(id, current)
		if err != nil {
			return "", err
		}
		defer links.Close()
		var (
			next int64
			found bool
			total int64 = 0
		)
		for links.Next() {
			if links.Err() != nil {
				return "", links.Err()
			}
			candidate := links.NextCountPair()
			total += int64(candidate.Count)
			if rand.Int64N(total) < int64(links.NextCountPair().Count) {
				next = candidate.Next
				found = true
			}
		}
		if !found {
			return "", fmt.Errorf("somehow failed to find next pair")
		}
		if next == 2 {
			break
		}
		tokens = append(tokens, next)
		current = next
	}
	words, err := wt.db.TranslateTokensToWords(tokens)
	if err != nil {
		return "", err
	}
	return strings.Join(words, " "), nil
}

