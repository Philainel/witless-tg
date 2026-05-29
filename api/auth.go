package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func (api *API) authHandler(w http.ResponseWriter, r *http.Request) {
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

	h := hmac.New(sha256.New, api.tg_verify_string)
	h.Write([]byte(dataCheckString))
	expectedHash := h.Sum(nil)
	providedHash, err := hex.DecodeString(hash)
	if err != nil {
		log.Println("failed to decode hash string: %s", err.Error())
		http.Error(w, "{}", http.StatusInternalServerError)
		return
	}
	if !hmac.Equal(expectedHash, providedHash) {
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
	signedToken, err := token.SignedString(api.private_key)
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

func (api *API) validateToken(r *http.Request) (jwt.MapClaims, error) {
	cookie, err := r.Cookie("auth-token")
	if err != nil {
		return nil, fmt.Errorf("missing auth token")
	}

	token, err := jwt.Parse(cookie.Value, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return api.public_key, nil
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

