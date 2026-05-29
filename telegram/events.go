package telegram

import (
	"log"
	"math/rand/v2"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func (tg *TG) sticker_message(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.EffectiveChat.Type == "private" || ctx.EffectiveChat.Type == "channel" {
		return nil
	}
	chance, mode, err := tg.db.GetChatById(ctx.EffectiveChat.Id)
	if err != nil { return err }
	if mode == "off" { return nil }

	if mode != "messaging" {
		if err := tg.wt.LearnSticker(ctx.EffectiveChat.Id, ctx.EffectiveMessage.Sticker.FileId); err != nil {
			log.Printf("error learning on sticker: %s\n", err.Error())
		}
	}

	isReplyToMe := ctx.EffectiveMessage.ReplyToMessage != nil && ctx.EffectiveMessage.ReplyToMessage.From.Id == b.User.Id
	if !isReplyToMe && (rand.Float64() > float64(chance / 1000) || mode == "learning") { // chance in ppm
		return  nil
	}

	text, err := tg.wt.Generate(ctx.EffectiveChat.Id)
	if err != nil {
		log.Printf("error generating message: %s\n", err.Error())
	}
	log.Printf("Generated message: %s\n", text)
	err = tg.handle_send(b, ctx, text, isReplyToMe)
	return err
}
func (tg *TG) text_message(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.EffectiveChat.Type == "private" || ctx.EffectiveChat.Type == "channel" {
		return nil
	}

	chance, mode, err := tg.db.GetChatById(ctx.EffectiveChat.Id)
	if err != nil { return err }
	if mode == "off" { return nil }

	if ctx.EffectiveMessage.Text[0] == '/' {
		return nil
	}

	if mode != "messaging" {
		err := tg.wt.Learn(ctx.EffectiveChat.Id, ctx.EffectiveMessage.Text)
		if err != nil {
			log.Printf("error learning on message: %s\n", err.Error())
		}
	}
	isReplyToMe := ctx.EffectiveMessage.ReplyToMessage != nil && ctx.EffectiveMessage.ReplyToMessage.From.Id == b.User.Id
	if isReplyToMe || (rand.Float64() <= float64(chance) / 1000.0 && mode != "learning") { // chance in ppm
		text, err := tg.wt.Generate(ctx.EffectiveChat.Id)
		if err != nil {
			log.Printf("error generating message: %s\n", err.Error())
		}
		log.Printf("Generated message: %s\n", text)
		err = tg.handle_send(b, ctx, text, isReplyToMe)
		return err
	}
	return nil
}
