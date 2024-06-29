package config

import (
	"context"
	"log"
	"os"

	"github.com/go-redis/redis/v8"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	"github.com/joho/godotenv"
)

var (
	DB  *gorm.DB
	RDB *redis.Client
	ctx = context.Background()
)

func init() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}
}

func Connect() {
	var err error
	dsn := os.Getenv("DB_DSN")
	DB, err = gorm.Open("postgres", dsn)
	if err != nil {
		panic("failed to connect to database")
	}

	RDB = redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_ADDR"),
		Username: os.Getenv("REDIS_USERNAME"),
		Password: os.Getenv("REDIS_PASSWORD"),
	})

	if err := RDB.Ping(ctx).Err(); err != nil {
		log.Fatalf("failed to connect to redis: %v", err)
	}
}

func GetDB() *gorm.DB {
	return DB
}

func GetRedis() *redis.Client {
	return RDB
}

func GetContext() context.Context {
	return ctx
}
