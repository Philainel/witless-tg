package main

import (
	"fmt"
	"os"
	"log"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
)

func main() {
	token := os.Getenv("TG_TOKEN")
	if token == "" {
		log.Fatal("TG_TOKEN not set")
	}
	b, err := gotgbot.NewBot(token, nil)
	if err != nil {
		log.Fatalf("can't create bot: %s", err.Error())
	}
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{
		Error: func(b *gotgbot.Bot, ctx *ext.Context, err error) ext.DispatcherAction {
			log.Println("an error occured while handling update:", err.Error())
			return ext.DispatcherActionNoop
		},
		MaxRoutines: ext.DefaultMaxRoutines,
	})
	updater := ext.NewUpdater(dispatcher, nil)
	dispatcher.AddHandler(handlers.NewCommand("start", start))
	dispatcher.AddHandler(handlers.NewMessage(message.Text, message))

	err = updater.StartPolling(b, &ext.PollingOpts{
		DropPendingUpdates: true,
		GetUpdatesOpts: &gotgbot.GetUpdatesOpts{
			Timeout: 9,
			RequestOpts: &gotgbot.RequestOpts{
				Timeout: time.Second * 10,
			},
		},
	})
	if err != nil {
		log.Fatalf("failed to start polling: %s", err.Error())
	}
	log.Printf("@%s has been started...\n", b.User.Username)

	updater.Idle()
}

func start(b *gotgbot.Bot, ctx *ext.Context) error {
	_, err := ctx.EffectiveMessage.Reply(b, 
		fmt.Sprintf(
			"Hello, I'm @%s.\nI am a sample bot to demonstrate how file sending works.\n\nTry the /source command!", 
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

func message(b *gotgbot.Bot, ctx *ext.Context) error {
	
}
