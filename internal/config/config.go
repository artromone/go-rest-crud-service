package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	HTTP     HTTP
	Postgres Postgres
}

type HTTP struct {
	Addr            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
}

type Postgres struct {
	Host     string
	Port     int
	User     string
	Password string
	DB       string
	SSLMode  string
}

func (p Postgres) DSN() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		p.User, p.Password, p.Host, p.Port, p.DB, p.SSLMode)
}

func Load() (Config, error) {
	_ = godotenv.Load()

	port, err := envInt("POSTGRES_PORT", 5433)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		HTTP: HTTP{
			Addr:            env("HTTP_ADDR", ":8080"),
			ReadTimeout:     envDuration("HTTP_READ_TIMEOUT", 5*time.Second),
			WriteTimeout:    envDuration("HTTP_WRITE_TIMEOUT", 10*time.Second),
			IdleTimeout:     envDuration("HTTP_IDLE_TIMEOUT", 120*time.Second),
			ShutdownTimeout: envDuration("HTTP_SHUTDOWN_TIMEOUT", 30*time.Second),
		},
		Postgres: Postgres{
			// 127.0.0.1, а не localhost: docker пробрасывает порт только
			// на IPv4, а localhost на многих системах резолвится в IPv6 ::1,
			// и подключение молча отваливается по таймауту.
			Host:     env("POSTGRES_HOST", "127.0.0.1"),
			Port:     port,
			User:     env("POSTGRES_USER", ""),
			Password: env("POSTGRES_PASSWORD", ""),
			DB:       env("POSTGRES_DB", ""),
			SSLMode:  env("POSTGRES_SSLMODE", "disable"),
		},
	}

	// У пароля и пользователя нет дефолта осознанно
	if cfg.Postgres.User == "" || cfg.Postgres.Password == "" || cfg.Postgres.DB == "" {
		return Config{}, fmt.Errorf(
			"POSTGRES_USER, POSTGRES_PASSWORD и POSTGRES_DB обязательны: скопируйте .env.example в .env")
	}
	return cfg, nil
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) (int, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return def, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be a number, got %q", key, raw)
	}
	return n, nil
}

func envDuration(key string, def time.Duration) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return def
	}
	if d, err := time.ParseDuration(raw); err == nil {
		return d
	}
	return def
}
