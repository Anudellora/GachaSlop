package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Environment          string
	HTTPAddr             string
	WebDir               string
	BannerHoYoBase       string
	BannerWuWaURL        string
	DatabaseDSN          string
	DatabaseMaxOpenConns int
	SyncWorkers          int
	SyncQueueSize        int
	OutboundTimeout      time.Duration
	ShutdownTimeout      time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		Environment:          env("APP_ENV", "development"),
		HTTPAddr:             env("HTTP_ADDR", "127.0.0.1:8080"),
		WebDir:               env("WEB_DIR", "web/dist"),
		BannerHoYoBase:       env("BANNER_HOYO_BASE_URL", "https://api.ennead.cc/mihoyo"),
		BannerWuWaURL:        env("BANNER_WUWA_URL", "https://aki-gm-resources-back.aki-game.com/gamenotice/G152/76402e5b20be2c39f095a152090afddc/en.json"),
		DatabaseDSN:          env("DATABASE_DSN", "file:data/gachaslop.db"),
		DatabaseMaxOpenConns: 4,
		SyncWorkers:          2,
		SyncQueueSize:        128,
		OutboundTimeout:      20 * time.Second,
		ShutdownTimeout:      10 * time.Second,
	}

	var err error
	if cfg.DatabaseMaxOpenConns, err = envInt("DATABASE_MAX_OPEN_CONNS", cfg.DatabaseMaxOpenConns); err != nil {
		return Config{}, err
	}
	if cfg.SyncWorkers, err = envInt("SYNC_WORKERS", cfg.SyncWorkers); err != nil {
		return Config{}, err
	}
	if cfg.SyncQueueSize, err = envInt("SYNC_QUEUE_SIZE", cfg.SyncQueueSize); err != nil {
		return Config{}, err
	}
	if cfg.OutboundTimeout, err = envDuration("OUTBOUND_TIMEOUT", cfg.OutboundTimeout); err != nil {
		return Config{}, err
	}
	if cfg.ShutdownTimeout, err = envDuration("SHUTDOWN_TIMEOUT", cfg.ShutdownTimeout); err != nil {
		return Config{}, err
	}

	if cfg.DatabaseMaxOpenConns < 1 || cfg.SyncWorkers < 1 || cfg.SyncQueueSize < 1 {
		return Config{}, fmt.Errorf("database connections, sync workers and queue size must be positive")
	}
	return cfg, nil
}

func env(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) (int, error) {
	raw := env(key, strconv.Itoa(fallback))
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", key, err)
	}
	return value, nil
}

func envDuration(key string, fallback time.Duration) (time.Duration, error) {
	raw := env(key, fallback.String())
	value, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", key, err)
	}
	return value, nil
}
