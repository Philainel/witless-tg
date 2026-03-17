package witless

import (
	"strings"

	"code.philainel.pw/philainel/witless-tg/core"
)

func (wt *Witless) Learn(id int64, text string) error {
	parts := strings.Split(text, " ")
	tokens, err := wt.db.GetTokensByWords(parts)
	if err != nil {
		return err
	}
	pairs := make([]*core.TokenPair, 0, len(parts) + 1)
	pairs = append(pairs, &core.TokenPair{Current: 1, Next: tokens[0]})
	for i := 0; i < len(tokens) - 1; i++ {
		pairs = append(pairs, &core.TokenPair{Current: tokens[i], Next: tokens[i+1]})
	}
	pairs = append(pairs, &core.TokenPair{Current: tokens[len(tokens)-1], Next: 2})
	wt.db.SaveLinksFromTokenPairs(pairs, id);
	return nil
}

func (wt *Witless) LearnSticker(id int64, sticker string) error {
	pairs := make([]*core.TokenPair, 2)
	tokens, err := wt.db.GetTokensByWords([]string{core.StickerStartMark + sticker + core.StickerEndMark})
	// log.Println(tokens)
	if err != nil {
		return err
	}
	pairs[0] = &core.TokenPair{Current: 1, Next: tokens[0]}
	pairs[1] = &core.TokenPair{Current: tokens[0], Next: 2}
	wt.db.SaveLinksFromTokenPairs(pairs, id)
	return nil
}


