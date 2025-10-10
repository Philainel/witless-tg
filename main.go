package main

import (
	"fmt"
	"os"
	"log"
	"time"
	"database/sql"
	"strings"
	"math/rand"
	"sync"
	"strconv"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
	"github.com/go-redis/redis/v8"
	"github.com/lib/pq"
)

var (
	db *sql.DB
	redisClient *redis.Client
	wipes sync.Map
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
	redisClient = redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		Password: "",
		DB: 0,
	})
	connection_string := "postgres://localhost:5432/postgres?sslmode=disable"
	db, err = sql.Open("postgres", connection_string)
	if err != nil {
		log.Fatalf("failed to open psql connection: %s", err.Error())
	}
	defer db.Close()
	query := `
		CREATE TABLE IF NOT EXISTS token (
			id BIGSERIAL PRIMARY KEY,
			word TEXT NOT NULL UNIQUE
		)
	`
	_, err = db.Exec(query)
	if err != nil {
		log.Fatalf("cannot create table token: %s", err.Error())
	}
	{
		query = `
			SELECT id FROM token WHERE id = 1
		`
		var id int64
		err := db.QueryRow(query).Scan(&id)
		if err == sql.ErrNoRows {
			query = `
				INSERT INTO token (word) VALUES (E'\x1f'), (E'\x1c')
			`
			_, err = db.Exec(query)
			if err != nil {
				log.Fatalf("can't create START and END tokens: %s", err.Error())
			}
		}
		if err != nil {
			log.Fatalf("can't ensure there's START and END tokens: %s", err.Error())
		}
	}
	query = `
		CREATE TABLE IF NOT EXISTS links (
			token BIGINT,
			chat BIGINT,
			next BIGINT,
			count INT,

			FOREIGN KEY (token) REFERENCES token(id),
			FOREIGN KEY (next) REFERENCES token(id),
			PRIMARY KEY (token, chat, next)
		)
	`
	_, err = db.Exec(query)
	if err != nil {
		log.Fatalf("cannot create table links: %s", err.Error())
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
	dispatcher.AddHandler(handlers.NewCommand("generate", generate_handler))
	dispatcher.AddHandler(handlers.NewCommand("wipe", wipe))
	dispatcher.AddHandler(handlers.NewMessage(message.Text, messages))

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
func wipe(b *gotgbot.Bot, ctx *ext.Context) error {
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
	data, ok := wipes.Load(ctx.EffectiveChat.Id)
	if !ok {
		code := rand.Intn(10000)
		wipes.Store(ctx.EffectiveChat.Id, ctx.EffectiveMessage.From.Id ^ int64(code))
		ctx.EffectiveMessage.Reply(b, fmt.Sprintf("⚠ ВНИМАНИЕ ⚠\n\nЭта команда навсегда (это очень долго) стирает данные Witless об этом чате!\nПосле удаления данные не подлежат восстановлению.\nДля подтверждения используйте команду `/wipe %04d` ещё раз в течение минуты", code), nil)
		return nil
	}
	code, err := strconv.Atoi(strings.Split(ctx.EffectiveMessage.Text, " ")[1])
	if err != nil {
		return nil
	}
	if ctx.EffectiveMessage.From.Id ^ int64(code) != data {
		return nil
	}
	query := `
		DELETE FROM links WHERE chat = $1
	`
	_, err = db.Exec(query, ctx.EffectiveChat.Id)
	if err != nil {
		return err
	}
	_, err = ctx.EffectiveMessage.SetReaction(b, &gotgbot.SetMessageReactionOpts{Reaction: []gotgbot.ReactionType{gotgbot.ReactionTypeEmoji{Emoji: "👌"}}})
	if err != nil {
		return err
	}
	return nil
}
func generate_handler(b *gotgbot.Bot, ctx *ext.Context) error {
	text, err := generate(ctx.EffectiveChat.Id)
	log.Printf("Generated message: %s\n", text)
	if err != nil {
		return fmt.Errorf("failed to send start message: %w", err)
	}
	_, err = ctx.EffectiveMessage.Reply(b, text, nil)
	if err != nil {
		return err
	}
	return nil
}
func messages(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.EffectiveChat.Type == "private" || ctx.EffectiveChat.Type == "channel" {
		return nil
	}
	// log.Println(ctx.EffectiveMessage.Text)
	if ctx.EffectiveMessage.Text[0] == '/' {
		return nil
	}
	err := learn(ctx.EffectiveChat.Id, ctx.EffectiveMessage.Text)
	if err != nil {
		log.Printf("error learning on message: %s\n", err.Error())
	}
	isReplyToMe := ctx.EffectiveMessage.ReplyToMessage != nil && ctx.EffectiveMessage.ReplyToMessage.From.Id == b.User.Id
	if !isReplyToMe && rand.Float64() > 0.20 { // 84%
		return  nil
	}
	// 16%
	text, err := generate(ctx.EffectiveChat.Id)
	if err != nil {
		log.Printf("error generating message: %s\n", err.Error())
	}
	log.Printf("Generated message: %s\n", text)
	if isReplyToMe {
		_, err = ctx.EffectiveMessage.Reply(b, text, nil)
		return err
	}
	_, err = ctx.EffectiveChat.SendMessage(b, text, nil)
	return err
}

type tokenpair struct {
	Current int64
	Next int64
}

func learn(id int64, text string) error {
	parts := strings.Split(text, " ")
	tokens, err := GetTokensByWords(parts)
	if err != nil {
		return err
	}
	pairs := make([]*tokenpair, 0, len(parts) + 1)
	pairs = append(pairs, &tokenpair{Current: 1, Next: tokens[0]})
	for i := 0; i < len(tokens) - 1; i++ {
		pairs = append(pairs, &tokenpair{Current: tokens[i], Next: tokens[i+1]})
	}
	pairs = append(pairs, &tokenpair{Current: tokens[len(tokens)-1], Next: 2})
	// for _, x := range pairs {
	// 	fmt.Printf("%d — %d; ", x.Current, x.Next)
	// }
	// fmt.Println();
	SaveLinksFromTokenPairs(pairs, id);
	return nil
}

func GetTokensByWords(words []string) ([]int64, error) {
	query := `
		SELECT id FROM token WHERE word = $1
	`
	insert := `
		INSERT INTO token (word) VALUES ($1) RETURNING id
	`
	var id int64
	result := make([]int64, 0, len(words))
	for _, w := range words {
		err := db.QueryRow(query, w).Scan(&id); 
		if err == sql.ErrNoRows {
			err = db.QueryRow(insert, w).Scan(&id);
		}
		if err != nil {
			return nil, err
		}
		result = append(result, id);
	}
	return result, nil
}

func SaveLinksFromTokenPairs(pairs []*tokenpair, id int64) {
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

type next_count struct {
	Next int64
	Count int
}

func generate(id int64) (string, error) {
	query := `
		SELECT next, count FROM links WHERE token = $1 AND chat = $2
	`
	var current int64 = 1
	tokens := make([]int64, 0, 10)
	for {
		rows, err := db.Query(query, current, id)
		if err != nil {
			return "", err
		}
		defer rows.Close()
		nexts := make([]*next_count, 0, 10)
		for rows.Next() {
			next := &next_count{}
			err := rows.Scan(&next.Next, &next.Count)
			if err != nil {
				return "", err
			}
			nexts = append(nexts, next)
		}
		// fmt.Printf("%d : ", current)
		// for _, n := range nexts {
		// 	fmt.Printf("%d (%d) ", n.Next, n.Count)
		// }
		// fmt.Printf("\n")
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
	// for _, t := range tokens {
	// 	fmt.Printf("%d, ", t)
	// }
	query = `
		SELECT id, word FROM token WHERE id = ANY($1)
	`
	rows, err := db.Query(query, pq.Array(tokens))
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
	// log.Println(words)
	return strings.Join(words, " "), nil
}

