package utils

import (
	"log"

	"github.com/go-redis/redis"
)

func CleanCacheById(redisClient *redis.Client, id string) {
	log.Println("Cleaning cache...")
	redisClient.Del(id)
	redisClient.Del("recipes")
	log.Println("Cache cleaned...")
}
