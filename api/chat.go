package api

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/PaulSonOfLars/gotgbot/v2"
)


func (api *API) getChatHandler(w http.ResponseWriter, r *http.Request) {
	chatIdInt, err := strconv.Atoi(r.URL.Query().Get("chat"))
	if (err != nil) {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	chatId := int64(chatIdInt)
	claims, err := api.validateToken(r);
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
	tgChat, err := api.tg.GetBot().GetChat(chatId, &gotgbot.GetChatOpts{})
	if err != nil {
		log.Printf("error fetching chat from Telegram: %s", err.Error())
		http.Error(w, "{}", http.StatusInternalServerError)
	}
	admins, err := tgChat.ToChat().GetAdministrators(api.tg.GetBot(), nil)
	editable := false
	if err != nil {
		http.Error(w, "{}", http.StatusInternalServerError)
		return
	}
	for i := range admins {
		if admins[i].GetUser().Id == data.Id {
			editable = admins[i].MergeChatMember().CanManageChat || admins[i].GetStatus() == "creator"
			break
		}
	}
	log.Println(data)
	log.Println(data.Id)
	if r.Method == "GET" {
		rate, mode, err := api.db.GetChatById(chatId)
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
			Editable: editable,
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
		if !editable {
			http.Error(w, "{}", http.StatusForbidden)
			return
		}
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
		err = api.db.ApplyChatSettingsById(chatId, parameters.Rate, parameters.Mode)
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

