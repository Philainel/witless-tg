package telegram

import (
	"math/rand"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func web_handler(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.EffectiveChat.Type == "channel" {
		return nil
	}
	if ctx.EffectiveChat.Type == "private" {
		_, err := ctx.EffectiveMessage.Reply(b, "Что-бы открыть панель управления бота, перейдите в группу с ботом и воспользуйтесь командой ещё раз", nil)
		return err
	}
	keyboard := [][]gotgbot.InlineKeyboardButton{{
		gotgbot.InlineKeyboardButton{
			Text: "Панель Управления",
			Url: fmt.Sprintf("https://t.me/%s/panel?startapp=%d", b.User.Username, ctx.EffectiveChat.Id),
		},
	}}
	_, err := ctx.EffectiveMessage.Reply(
		b,
		"Нажмите на кнопку ниже, чтобы открыть Панель Управления:",
		&gotgbot.SendMessageOpts{
		ReplyMarkup: gotgbot.InlineKeyboardMarkup{
				InlineKeyboard: keyboard,
			},
		},
	)

	return err
}

func (tg *TG) start_handler(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.EffectiveChat.Type == gotgbot.ChatTypeGroup || ctx.EffectiveChat.Type == gotgbot.ChatTypeSupergroup {
		err := tg.db.InitDefaultSettings(ctx.EffectiveChat.Id) // TODO: move to core.witless
		if err != nil {
			log.Printf("failed to ensure chat settings record exists: %s", err.Error())
		}
	}
	_, err := ctx.EffectiveMessage.Reply(b, 
		fmt.Sprintf(
			"Привет! Я @%s — Реинкарнация бота из VK.\n\nЯ работаю только в групповых чатах, где обучаюсь на сообщениях,  затем сам начинаю писать разные приколы.", 
			b.User.Username,
		), 
		&gotgbot.SendMessageOpts{
			ParseMode: "HTML",
	})
	if err != nil {
		return fmt.Errorf("failed to send start message: %w", err)
	}
	return nil
}

func (tg *TG) wipe_handler(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.EffectiveChat.Type == "private" || ctx.EffectiveChat.Type == "channel" {
		return nil
	}
	admins, err := ctx.EffectiveChat.GetAdministrators(b, nil)
	if err != nil {
		return err
	}
	result := false
	for i := range admins {
		if admins[i].GetUser().Id == ctx.EffectiveMessage.From.Id {
			result = admins[i].MergeChatMember().CanDeleteMessages || admins[i].GetStatus() == "creator"
			break
		}
	}
	if !result {
		return nil
	}
	data, ok := tg.wipes.Load(ctx.EffectiveChat.Id)
	if !ok {
		code := rand.Intn(10000)
		tg.wipes.Store(ctx.EffectiveChat.Id, ctx.EffectiveMessage.From.Id ^ int64(code))
		_, err = ctx.EffectiveMessage.Reply(b, fmt.Sprintf("⚠ ВНИМАНИЕ ⚠\n\nЭта команда навсегда (это очень долго) стирает данные Witless об этом чате!\nПосле удаления данные не подлежат восстановлению.\nДля подтверждения используйте команду `/wipe %04d` ещё раз в течение минуты", code), &gotgbot.SendMessageOpts{ParseMode: "Markdown"})
		return err
	}
	code, err := strconv.Atoi(strings.Split(ctx.EffectiveMessage.Text, " ")[1])
	if err != nil {
		return err
	}
	if ctx.EffectiveMessage.From.Id ^ int64(code) != data {
		return nil
	}
	query := `
		DELETE FROM links WHERE chat = $1
	`
	_, err = tg.db.GetDB().Exec(query, ctx.EffectiveChat.Id) // move away from GetDB
	if err != nil {
		return err
	}
	err = tg.db.ResetToDefaultSettings(ctx.EffectiveChat.Id)
	if err != nil { return err }
	_, err = ctx.EffectiveMessage.SetReaction(b, 
		&gotgbot.SetMessageReactionOpts{
			Reaction: []gotgbot.ReactionType{
				gotgbot.ReactionTypeEmoji{
					Emoji: "👌",
				},
			},
		},
	)
	if err != nil {
		return err
	}
	return nil
}

func (tg *TG) generate_handler(b *gotgbot.Bot, ctx *ext.Context) error {
	text, err := tg.wt.Generate(ctx.EffectiveChat.Id)
	log.Printf("Generated message: %s\n", text)
	if err != nil {
		return fmt.Errorf("failed to send start message: %w", err)
	}
	err = tg.handle_send(b, ctx, text, true)
	return err
}

