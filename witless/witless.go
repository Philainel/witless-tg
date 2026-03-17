package witless

import (
	"github.com/go-redis/redis/v8"

	database "code.philainel.pw/philainel/witless-tg/db"
)

type Witless struct {
	db *database.DB
	redisClient *redis.Client
}

func NewWitless(db *database.DB, redisClient *redis.Client) *Witless {
	return &Witless{ db: db, redisClient: redisClient }
}

