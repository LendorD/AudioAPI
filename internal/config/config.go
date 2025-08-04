package config

import (
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"
)

type ServerConfig struct {
	Port           string
	AllowedOrigins []string
}

type Config struct {
	App        AppConfig
	HTTPServer HTTPConfig
	Services   Services
	Server     ServerConfig // Добавляем ServerConfig в основную структуру
	JWTSecret  string
}

func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		Port: getEnv("SERVER_PORT", "8080"),
		AllowedOrigins: []string{
			"http://localhost:3000",
			"http://localhost:5173",
			"http://localhost:4200",
			"http://localhost:8081",
			"http://127.0.0.1:3000",
			"http://127.0.0.1:5173",
		},
	}
}

type AppConfig struct {
	Version string
}

type HTTPConfig struct {
	Port              string
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
}
type Services struct {
	MobileApp Service
}

type Service struct {
	Host string
}

func LoadConfig() (*Config, error) {
	err := godotenv.Load()
	if err != nil {
		return nil, fmt.Errorf("error loading .env file: %s", err.Error())
	}

	// Инициализируем Config с Server
	cfg := &Config{
		App: AppConfig{
			Version: getEnv("VERSION", "1.0.0"),
		},
		HTTPServer: HTTPConfig{
			Port:              getEnv("SERVER_PORT", "6004"),
			ReadTimeout:       time.Second * 10,
			ReadHeaderTimeout: time.Second * 20,
			WriteTimeout:      time.Second * 20,
		},
		Services: Services{
			MobileApp: Service{
				Host: getEnv("API_URL", "http://localhost:8080"),
			},
		},
		Server: ServerConfig{ // Явно инициализируем Server
			Port: getEnv("SERVER_PORT", "8080"),
			AllowedOrigins: []string{
				"http://localhost:3000",
				"http://localhost:5173",
				"http://localhost:4200",
				"http://localhost:8081",
				"http://127.0.0.1:3000",
				"http://127.0.0.1:5173",
			},
		},
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
