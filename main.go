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
	"sort"
	"strconv"
	"net/http"
	"net/url"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"crypto/rsa"
	"io"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
	"github.com/go-redis/redis/v8"
	"github.com/lib/pq"
	"github.com/golang-jwt/jwt/v5"

	database "code.philainel.pw/philainel/witless-tg/db"
)

var (
	db *sql.DB
	redisClient *redis.Client
	bot *gotgbot.Bot
	wipes sync.Map
	webs sync.Map
	token string
	private *rsa.PrivateKey
	public *rsa.PublicKey
)

func main() {
	token = os.Getenv("TG_TOKEN")
	if token == "" {
		log.Fatal("TG_TOKEN not set")
	}
	b, err := gotgbot.NewBot(token, nil)
	if err != nil {
		log.Fatalf("can't create bot: %s", err.Error())
	}
	bot = b
	pubKeyLoc := os.Getenv("PUBLIC_KEY_PATH")
	if pubKeyLoc == "" {
		log.Fatal("PUBLIC_KEY_PATH not set")
	}
	privKeyLoc := os.Getenv("PRIVATE_KEY_PATH")
	if privKeyLoc == "" {
		log.Fatal("PRIVATE_KEY_PATH not set")
	}
	publicPem, err := os.ReadFile(pubKeyLoc)
	if err != nil {
		log.Fatalf("error reading file %s: %s", pubKeyLoc, err.Error())
	}
	privatePem, err := os.ReadFile(privKeyLoc)
	if err != nil {
		log.Fatalf("error reading file %s: %s", privKeyLoc, err.Error())
	}
	public, err = jwt.ParseRSAPublicKeyFromPEM(publicPem)
	if err != nil {
		log.Fatalf("can't parse public key: %s", err.Error())
	}
	private, err = jwt.ParseRSAPrivateKeyFromPEM(privatePem)
	if err != nil {
		log.Fatalf("can't parse private key: %s", err.Error())
	}
	redisHost := os.Getenv("REDIS_HOST")
	redisClient = redis.NewClient(&redis.Options{
		Addr: redisHost,
		Password: "",
		DB: 0,
	})
	host := os.Getenv("POSTGRES_HOST")
	user := os.Getenv("POSTGRES_USER")
	password := os.Getenv("POSTGRES_PASSWORD")
	dbname := os.Getenv("POSTGRES_DB")
	connection_string := fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable", user, password, host, dbname)
	db, err = sql.Open("postgres", connection_string)
	if err != nil {
		log.Fatalf("failed to open psql connection: %s", err.Error())
	}
	defer db.Close()
	err = database.PerformMigration(db, 2)
	if err != nil {
		log.Fatalf("cannot perform migration: %s", err.Error())
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
	dispatcher.AddHandler(handlers.NewCommand("web", web_handler))
	dispatcher.AddHandler(handlers.NewMessage(message.Text, messages))
	dispatcher.AddHandler(handlers.NewMessage(message.Sticker, stickers))

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
	log.Printf("@%s has been started\n", b.User.Username)
	
	go api_server()

	updater.Idle()
}

func web_handler(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.EffectiveChat.Type == "channel" {
		return nil
	}
	if ctx.EffectiveChat.Type == "private" {
		parts := strings.Split(ctx.EffectiveMessage.Text, " ")
		if len(parts) < 2 {
			_, err := ctx.EffectiveMessage.Reply(b, "Что-бы открыть панель управления бота, перейдите в группу с ботом и воспользуйтесь командой ещё раз", nil)
			return err
		}
		code, err := strconv.Atoi(parts[1])
		if err != nil {
			return err
		}
		// TODO: code check
		raw, ok := webs.Load(ctx.EffectiveMessage.From.Id)
		if !ok {
			_, err := ctx.EffectiveMessage.Reply(b, "Что-бы открыть панель управления бота, перейдите в группу с ботом и воспользуйтесь командой ещё раз", nil)
			return err
		}
		data, ok := raw.(int64)
		if !ok {
			return fmt.Errorf("How did we get here? (/web raw->int64 failed)")
		}
		chatId := data ^ int64(code)
		chat, err := b.GetChat(chatId, &gotgbot.GetChatOpts{})
		if err != nil {
			return err
		}
		admins, err := chat.ToChat().GetAdministrators(b, nil)
		if err != nil {
			return err
		}
		result := false
		for i := range admins {
			if admins[i].GetUser().Id == ctx.EffectiveMessage.From.Id {
				result = admins[i].MergeChatMember().CanManageChat || admins[i].GetStatus() == "creator"
				break
			}
		}
		if !result {
			return nil
		}
	
		keyboard := [][]gotgbot.InlineKeyboardButton{{
			gotgbot.InlineKeyboardButton{
				Text: "Панель Управления",
				WebApp: &gotgbot.WebAppInfo{
					Url: fmt.Sprintf("%s?chat=%d", os.Getenv("WEB_APP_URL"), chatId),
				},
			},
		}}
		_, err = ctx.EffectiveMessage.Reply(
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
	// TODO: code gen
	admins, err := ctx.EffectiveChat.GetAdministrators(b, nil)
	if err != nil {
		return err
	}
	result := false
	for i := range admins {
		if admins[i].GetUser().Id == ctx.EffectiveMessage.From.Id {
			result = admins[i].MergeChatMember().CanManageChat || admins[i].GetStatus() == "creator"
			break
		}
	}
	if !result {
		return nil
	}
	code := rand.Intn(10000)
	webs.Store(ctx.EffectiveMessage.From.Id, ctx.EffectiveChat.Id ^ int64(code))
	// _, err = ctx.EffectiveMessage.Reply(b, fmt.Sprintf("Чтобы открыть панель управления бота в этом чате, перейдите в личные сообщения и введите эту команду `/web %04d`", code), &gotgbot.SendMessageOpts{ParseMode: "Markdown"})
	keyboard := [][]gotgbot.InlineKeyboardButton{{
		gotgbot.InlineKeyboardButton{
			Text: "Панель Управления",
			Url: fmt.Sprintf("https://t.me/%s/panel?startapp=%d", b.User.Username, ctx.EffectiveChat.Id),
		},
	}}
	_, err = ctx.EffectiveMessage.Reply(
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

func api_server() {
	http.HandleFunc("/auth", authHandler)
	http.HandleFunc("/chat", getChatHandler)
	http.Handle("/", http.FileServer(http.Dir("./web")))
	log.Println("Serving API server on 0.0.0.0:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		panic(err);
	}
}

func authHandler(w http.ResponseWriter, r *http.Request) {
	data := r.URL.Query().Get("data")
	if data == "" {
		http.Error(w, "missing data", http.StatusBadRequest)
		return
	}
	values, err := url.ParseQuery(data)
	if err != nil {
		http.Error(w, "invalid data", http.StatusBadRequest)
		return
	}
	hash := values.Get("hash")
	values.Del("hash")
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var dataCheckStrings []string
	for _, k := range keys {
		dataCheckStrings = append(dataCheckStrings, k+"="+values.Get(k))
	}
	dataCheckString := strings.Join(dataCheckStrings, "\n")
	h := hmac.New(sha256.New, []byte("WebAppData"))
	h.Write([]byte(token))
	secretKey := h.Sum(nil)
	h2 := hmac.New(sha256.New, secretKey)
	h2.Write([]byte(dataCheckString))
	expectedHash := hex.EncodeToString(h2.Sum(nil))
	if !hmac.Equal([]byte(expectedHash), []byte(hash)){
		http.Error(w, "invalid hash", http.StatusUnauthorized)
		return
	}
	authDateStr := values.Get("auth_date")
	if authDateStr != "" {
		sec, _ := time.ParseDuration(authDateStr + "s")
		if time.Since(time.Unix(int64(sec.Seconds()), 0)) > 24*time.Hour {
			http.Error(w, "data expired", http.StatusUnauthorized)
			return
		}
	}
	claims := jwt.MapClaims{
		"user": values.Get("user"),
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signedToken, err := token.SignedString(private)
	if err != nil {
		http.Error(w, "failed to sign token", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name: "auth-token",
		Value: signedToken,
		Path: "/",
		HttpOnly: true,
		Secure: true,
		Partitioned: true,
		SameSite: http.SameSiteNoneMode,
	})

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"ok":true,"message":"Auth successful","user":%s}`, values.Get("user"))
}

type User struct {
	Id int64 `json:"id"`
	FirstName string `json:"first_name"`
	LastName string `json:"last_name"`
	Username string `json:"username"`
	LanguageCode string `json:"language_code"`
	PhotoUrl string `json:"photo_url"`
}

type chatData struct {
	Id int64 `json:"id"`;
	Title string `json:"title"`;
	Photo *gotgbot.ChatPhoto `json:"photo"`;
	Settings *chatSettings `json:"settings"`
}

type chatSettings struct {
	Rate int16 `json:"rate"`;
	Mode string `json:"mode"`
}

func getChatHandler(w http.ResponseWriter, r *http.Request) {
	chatIdInt, err := strconv.Atoi(r.URL.Query().Get("chat"))
	if (err != nil) {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	chatId := int64(chatIdInt)
	claims, err := validateToken(r);
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	var data User
	user, ok := claims["user"].(string)
	if !ok {
		log.Println("error convarting claim 'user' to string")
		http.Error(w, "{}", http.StatusInternalServerError)
		return
	}
	if err := json.Unmarshal([]byte(user), &data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tgChat, err := bot.GetChat(chatId, &gotgbot.GetChatOpts{})
	if err != nil {
		log.Printf("error fetching chat from Telegram: %s", err.Error())
		http.Error(w, "{}", http.StatusInternalServerError)
	}
	log.Println(data)
	log.Println(data.Id)
	if r.Method == "GET" {
		rate, mode, err := database.GetChatById(db, chatId)
		if err != nil {
			log.Printf("error db.GetChatById(...): %s", err.Error())
			http.Error(w, "{}", http.StatusInternalServerError)
			return
		}
		chat := &chatData{
			Id: chatId,
			Photo: tgChat.Photo,
			Title: tgChat.Title,
			Settings: &chatSettings {
				Rate: rate,
				Mode: mode,
			},
		}
		res, err := json.Marshal(chat)
		if err != nil {
			log.Printf("failed to marashal json for reply: %s", err.Error())
			http.Error(w, "{}", http.StatusInternalServerError)
			return
		}
		w.Write([]byte(res))
		return
	}
	if (r.Method == "POST") {
		var parameters chatSettings
		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.Printf("failed to read r.Body: %s", err.Error())
			http.Error(w, "{}", http.StatusInternalServerError)
			return
		}
		log.Println(string(body))
		err = json.Unmarshal(body, &parameters)
		if err != nil {
			log.Printf("json.Unmarshal(body, &parameters) failed: %s", err.Error())
			http.Error(w, "{}", http.StatusInternalServerError)
			return
		}
		log.Println(parameters)
		err = database.ApplyChatSettingsById(db, chatId, parameters.Rate, parameters.Mode)
		if err != nil {
			log.Printf("failed to apply settings (rate %.3d, mode %s) to chat %d: %s", parameters.Rate, parameters.Mode, chatId, err.Error())
			http.Error(w, "{}", http.StatusInternalServerError)
			return
		}
		w.Write([]byte("{\"ok\": true}"))
		return
	}
	http.Error(w, "{}", http.StatusBadRequest)
}

func validateToken(r *http.Request) (jwt.MapClaims, error) {
	cookie, err := r.Cookie("auth-token")
	if err != nil {
		return nil, fmt.Errorf("missing auth token")
	}

	token, err := jwt.Parse(cookie.Value, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return public, nil
	})
	if err != nil {
		return nil, fmt.Errorf("invalid token: %s", err.Error())
	}

	if !token.Valid {
		return nil, fmt.Errorf("token not valid")
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		if exp, ok := claims["exp"].(float64); ok {
			if time.Now().Unix() > int64(exp) {
				return nil, fmt.Errorf("token expired")
			}
			return claims, nil
		} else {
			return nil, fmt.Errorf("missing exp claim")
		}
	} else {
		return nil, fmt.Errorf("invalid claims format")
	}
}

func start(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.EffectiveChat.Type == gotgbot.ChatTypeGroup || ctx.EffectiveChat.Type == gotgbot.ChatTypeSupergroup {
		err := database.InitDefaultSettings(db, ctx.EffectiveChat.Id)
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
func wipe(b *gotgbot.Bot, ctx *ext.Context) error {
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
	data, ok := wipes.Load(ctx.EffectiveChat.Id)
	if !ok {
		code := rand.Intn(10000)
		wipes.Store(ctx.EffectiveChat.Id, ctx.EffectiveMessage.From.Id ^ int64(code))
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
	_, err = db.Exec(query, ctx.EffectiveChat.Id)
	if err != nil {
		return err
	}
	err = database.ResetToDefaultSettings(db, ctx.EffectiveChat.Id)
	if err != nil { return err }
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
	err = handle_send(b, ctx, text, true)
	return err
}

func stickers(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.EffectiveChat.Type == "private" || ctx.EffectiveChat.Type == "channel" {
		return nil
	}
	chance, mode, err := database.GetChatById(db, ctx.EffectiveChat.Id)
	if err != nil { return err }
	if mode == "off" { return nil }

	if mode != "messaging" {
		if err := learn_sticker(ctx.EffectiveChat.Id, ctx.EffectiveMessage.Sticker.FileId); err != nil {
			log.Printf("error learning on sticker: %s\n", err.Error())
		}
	}

	isReplyToMe := ctx.EffectiveMessage.ReplyToMessage != nil && ctx.EffectiveMessage.ReplyToMessage.From.Id == b.User.Id
	if !isReplyToMe && (rand.Float64() > float64(chance / 1000) || mode == "learning") { // chance in ppm
		return  nil
	}

	text, err := generate(ctx.EffectiveChat.Id)
	if err != nil {
		log.Printf("error generating message: %s\n", err.Error())
	}
	log.Printf("Generated message: %s\n", text)
	err = handle_send(b, ctx, text, isReplyToMe)
	return err
}
func messages(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.EffectiveChat.Type == "private" || ctx.EffectiveChat.Type == "channel" {
		return nil
	}

	chance, mode, err := database.GetChatById(db, ctx.EffectiveChat.Id)
	if err != nil { return err }
	if mode == "off" { return nil }

	if ctx.EffectiveMessage.Text[0] == '/' {
		return nil
	}

	if mode != "messaging" {
		err := learn(ctx.EffectiveChat.Id, ctx.EffectiveMessage.Text)
		if err != nil {
			log.Printf("error learning on message: %s\n", err.Error())
		}
	}
	isReplyToMe := ctx.EffectiveMessage.ReplyToMessage != nil && ctx.EffectiveMessage.ReplyToMessage.From.Id == b.User.Id
	if isReplyToMe || (rand.Float64() <= float64(chance) / 1000.0 && mode != "learning") { // chance in ppm
		text, err := generate(ctx.EffectiveChat.Id)
		if err != nil {
			log.Printf("error generating message: %s\n", err.Error())
		}
		log.Printf("Generated message: %s\n", text)
		err = handle_send(b, ctx, text, isReplyToMe)
		return err
	}
	return nil
}

const (
	sticker_start_mark = "E'\x1d"
	sticker_end_mark   = "'"
)


func handle_send(b *gotgbot.Bot, ctx *ext.Context, text string, reply bool) error {
	if strings.HasPrefix(text, sticker_start_mark) && strings.HasSuffix(text, sticker_end_mark) {
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
	SaveLinksFromTokenPairs(pairs, id);
	return nil
}

func learn_sticker(id int64, sticker string) error {
	pairs := make([]*tokenpair, 2)
	tokens, err := GetTokensByWords([]string{sticker_start_mark + sticker + sticker_end_mark})
	log.Println(tokens)
	if err != nil {
		return err
	}
	pairs[0] = &tokenpair{Current: 1, Next: tokens[0]}
	pairs[1] = &tokenpair{Current: tokens[0], Next: 2}
	SaveLinksFromTokenPairs(pairs, id)
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

