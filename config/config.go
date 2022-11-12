package config

import (
	"os"
	"sync"
)

type DataBaseConfig struct {
	User    string
	Pass    string
	Name    string
	SSLMode string
	Driver  string
}

type Config struct {
	DataBase DataBaseConfig
}

var newConfig *Config
var once sync.Once

func New() *Config {
	once.Do(func() {
		newConfig = &Config{
			DataBase: DataBaseConfig{
				User:    getEnv("DATABASE_USER", ""),
				Pass:    getEnv("DATABASE_PASS", ""),
				Name:    getEnv("DATABASE_NAME", ""),
				SSLMode: getEnv("DATABASE_SSLMODE", ""),
				Driver:  getEnv("DATABASE_DRIVER", ""),
			},
		}
	})
	return newConfig
}

func getEnv(key string, defaultVal string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultVal
}
