package telegram

import (
	"log"
	"strings"
	"sync"
	"time"

	"code.philainel.pw/philainel/witless-tg/core"
	database "code.philainel.pw/philainel/witless-tg/db"
	"code.philainel.pw/philainel/witless-tg/witless"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
)

type TG struct {
	bot *gotgbot.Bot
	wt *witless.Witless
	db *database.DB
	updater *ext.Updater
	wipes sync.Map
}

func NewTG(token string, wt *witless.Witless, db *database.DB) (*TG, error) {
	b, err := gotgbot.NewBot(token, nil)
	return &TG{bot: b, wt: wt, db: db}, err
}

func (tg *TG) GetBot() *gotgbot.Bot { // TODO: Encapsulate bot
	return tg.bot
}

func (tg *TG) RegisterHandlers() {
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{
		Error: func(b *gotgbot.Bot, ctx *ext.Context, err error) ext.DispatcherAction {
			log.Println("an error occured while handling update:", err.Error())
			return ext.DispatcherActionNoop
		},
		MaxRoutines: ext.DefaultMaxRoutines,
	})
	tg.updater = ext.NewUpdater(dispatcher, nil)
	dispatcher.AddHandler(handlers.NewCommand("start", tg.start_handler))
	dispatcher.AddHandler(handlers.NewCommand("generate", tg.generate_handler))
	dispatcher.AddHandler(handlers.NewCommand("wipe", tg.wipe_handler))
	dispatcher.AddHandler(handlers.NewCommand("web", web_handler))
	dispatcher.AddHandler(handlers.NewMessage(message.Text, tg.text_message))
	dispatcher.AddHandler(handlers.NewMessage(message.Sticker, tg.sticker_message))
}

func (tg *TG) EventLoop() error {
	err := tg.updater.StartPolling(tg.bot, &ext.PollingOpts{
		DropPendingUpdates: true,
		GetUpdatesOpts: &gotgbot.GetUpdatesOpts{
			Timeout: 9,
			RequestOpts: &gotgbot.RequestOpts{
				Timeout: time.Second * 10,
			},
		},
	})
	if err != nil { return err };
	log.Printf("@%s has been started\n", tg.bot.User.Username)
	tg.updater.Idle()
	return nil
}

// factor out to separate file
func (tg *TG) handle_send(b *gotgbot.Bot, ctx *ext.Context, text string, reply bool) error {
	if strings.HasPrefix(text, core.StickerStartMark) && strings.HasSuffix(text, core.StickerEndMark) {
		sticker := gotgbot.InputFileByID(text[3:len(text)-1])
		var replyParameters *gotgbot.ReplyParameters
		if reply {
			replyParameters = &gotgbot.ReplyParameters{
				MessageId: ctx.EffectiveMessage.MessageId,
			}
		}
		_, err := b.SendSticker(ctx.EffectiveChat.Id, sticker, &gotgbot.SendStickerOpts{
			ReplyParameters: replyParameters,
		})
		return err
	}
	if reply {
		_, err := ctx.EffectiveMessage.Reply(b, text, nil)
		return err
	}
	_, err := ctx.EffectiveChat.SendMessage(b, text, nil)
	return err

}

