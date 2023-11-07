package db

import (
	"context"
	"github.com/go-redis/redis/v8"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"shyIM/config"
	"shyIM/pkg/logger"
)

var (
	DB  *gorm.DB
	RDB *redis.Client
)

func InitDB() (err error) {
	DB, err = gorm.Open(mysql.Open(config.GlobalConfig.MySQL.DNS), &gorm.Config{})
	if err != nil {
		logger.Slog.Error("Failed to connect to mysql ", "[ERROR]", err)
		return
	}
	logger.Slog.Info("Initializing mysql successfully")

	RDB = redis.NewClient(&redis.Options{
		Addr:         config.GlobalConfig.Redis.Address,
		DB:           0,
		Password:     config.GlobalConfig.Redis.Password,
		PoolSize:     30,
		MinIdleConns: 30,
	})
	err = RDB.Ping(context.Background()).Err()
	if err != nil {
		logger.Slog.Error("Failed to connect to redis ", "[ERROR]", err)
		return
	}
	logger.Slog.Info("Initializing redis successfully")
	return
}
