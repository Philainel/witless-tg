package main

import (
	"log"
	"os"

	"github.com/go-redis/redis/v8"

	"code.philainel.pw/philainel/witless-tg/api"
	database "code.philainel.pw/philainel/witless-tg/db"
	"code.philainel.pw/philainel/witless-tg/telegram"
	"code.philainel.pw/philainel/witless-tg/witless"
)

func main() {
	pubKeyLoc := os.Getenv("PUBLIC_KEY_PATH")
	if pubKeyLoc == "" { log.Fatal("PUBLIC_KEY_PATH not set") }
	privKeyLoc := os.Getenv("PRIVATE_KEY_PATH")
	if privKeyLoc == "" { log.Fatal("PRIVATE_KEY_PATH not set") }

	publicPem, err := os.ReadFile(pubKeyLoc)
	if err != nil { log.Fatalf("error reading file %s: %s", pubKeyLoc, err.Error()) }
	privatePem, err := os.ReadFile(privKeyLoc)
	if err != nil { log.Fatalf("error reading file %s: %s", privKeyLoc, err.Error()) }

	redisClient := redis.NewClient(&redis.Options{
		Addr: os.Getenv("REDIS_HOST"),
		Password: "",
		DB: 0,
	})
	db, err := database.NewDB(
		os.Getenv("POSTGRES_USER"),
		os.Getenv("POSTGRES_PASSWORD"),
		os.Getenv("POSTGRES_HOST"),
		os.Getenv("POSTGRES_DB"),
	)
	if err != nil {
		log.Fatalf("failed to open db connection: %s", err.Error())
	}
	defer db.Close()
	err = db.PerformMigration(2)
	if err != nil { log.Fatalf("failed to perform migration: %s", err.Error()) }

	wt := witless.NewWitless(db, redisClient)

	token := os.Getenv("TG_TOKEN")
	if token == "" { log.Fatal("TG_TOKEN not set") }

	tg, err := telegram.NewTG(token, wt, db)
	if err != nil { log.Fatalf("failed to create bot: %s", err.Error()) }
	tg.RegisterHandlers()

	server := api.NewAPI(tg, db, publicPem, privatePem)
	server.BakeTgVerifyString(token)
	go server.ListenAndServe()

	err = tg.EventLoop()
	if err != nil {
		log.Fatalf("failed to start polling: %s", err.Error())
	}
}

