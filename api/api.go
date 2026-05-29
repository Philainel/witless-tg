package api

import (
	"crypto/hmac"
	"crypto/rsa"
	"crypto/sha256"
	"log"
	"net/http"

	database "code.philainel.pw/philainel/witless-tg/db"
	"code.philainel.pw/philainel/witless-tg/telegram"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/golang-jwt/jwt/v5"
)

type API struct {
	db *database.DB
	tg *telegram.TG
	public_key *rsa.PublicKey
	private_key *rsa.PrivateKey
	tg_verify_string []byte
}

func NewAPI(tg *telegram.TG, db *database.DB, public_key_pem, private_key_pem []byte) *API {
	public, err := jwt.ParseRSAPublicKeyFromPEM(public_key_pem)
	if err != nil { log.Fatalf("failed to parse public key: %s", err.Error()) }

	private, err := jwt.ParseRSAPrivateKeyFromPEM(private_key_pem)
	if err != nil { log.Fatalf("failed to parse private key: %s", err.Error()) }

	return &API{ db: db, tg: tg, public_key: public, private_key: private}
}

func (api *API) BakeTgVerifyString(token string) {
	h := hmac.New(sha256.New, []byte("WebAppData"))
	h.Write([]byte(token))
	api.tg_verify_string = h.Sum(nil)
}

func (api *API) ListenAndServe() {
	http.HandleFunc("/auth", api.authHandler)
	http.HandleFunc("/chat", api.getChatHandler)
	http.Handle("/", http.FileServer(http.Dir("./web")))
	log.Println("Serving API server on 0.0.0.0:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		panic(err);
	}
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
	Id int64 `json:"id"`
	Title string `json:"title"`
	Photo *gotgbot.ChatPhoto `json:"photo"`
	Settings *chatSettings `json:"settings"`
	Editable bool `json:"editable"`
}

type chatSettings struct {
	Rate int16 `json:"rate"`
	Mode string `json:"mode"`
}


