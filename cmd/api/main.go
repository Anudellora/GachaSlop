package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"gachaslop/internal/adapters/banners"
	"gachaslop/internal/adapters/gacha"
	"gachaslop/internal/adapters/gacha/hoyo"
	"gachaslop/internal/adapters/gacha/wuwa"
	"gachaslop/internal/adapters/httpapi"
	"gachaslop/internal/adapters/queue"
	"gachaslop/internal/adapters/storage/sqlite"
	"gachaslop/internal/adapters/webui"
	"gachaslop/internal/application"
	"gachaslop/internal/config"
	"gachaslop/internal/domain"
	platform "gachaslop/internal/platform/id"
)

func main() {
	if err := run(); err != nil {
		slog.Error("application stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	logger := newLogger(cfg.Environment)
	slog.SetDefault(logger)

	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	store, err := sqlite.Open(rootCtx, cfg.DatabaseDSN, cfg.DatabaseMaxOpenConns)
	if err != nil {
		return err
	}
	defer store.Close()
	clock := platform.Clock{}
	if err := store.FailInterruptedSyncJobs(rootCtx, clock.Now()); err != nil {
		return err
	}

	httpClient := &http.Client{
		Timeout: cfg.OutboundTimeout,
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			DialContext:           (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   10,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   5 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
		},
	}
	genshin, err := hoyo.New(domain.GameGenshin, httpClient, hoyo.Options{Delay: 150 * time.Millisecond})
	if err != nil {
		return err
	}
	hsr, err := hoyo.New(domain.GameHSR, httpClient, hoyo.Options{Delay: 150 * time.Millisecond})
	if err != nil {
		return err
	}
	zzz, err := hoyo.New(domain.GameZZZ, httpClient, hoyo.Options{Delay: 150 * time.Millisecond})
	if err != nil {
		return err
	}
	sources := gacha.Registry{
		domain.GameGenshin: genshin,
		domain.GameHSR:     hsr,
		domain.GameZZZ:     zzz,
		domain.GameWuWa:    wuwa.New(httpClient, 300*time.Millisecond),
	}

	ids := platform.Generator{}
	syncQueue := queue.NewMemory(cfg.SyncQueueSize)
	service := application.NewService(store, sources, syncQueue, ids, clock, logger)
	service.StartWorkers(rootCtx, cfg.SyncWorkers)

	bannerService := application.NewBannerService(store, banners.New(httpClient, cfg.BannerHoYoBase, cfg.BannerWuWaURL), clock, 15*time.Minute)
	bannerService.WithArtwork(banners.NewArtwork(httpClient, banners.ArtworkCatalogBase))
	bannerService.Start(rootCtx)
	api := httpapi.New(service, store, logger, ids).WithBanners(bannerService)
	handler := api.Handler()
	if _, err := os.Stat(filepath.Join(cfg.WebDir, "index.html")); err == nil {
		handler = webui.Handler(handler, os.DirFS(cfg.WebDir))
		logger.Info("web frontend enabled", "directory", cfg.WebDir)
	} else if os.IsNotExist(err) {
		logger.Info("API-only mode; build frontend with npm --prefix web run build")
	} else {
		return err
	}
	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}

	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("http server started", "address", cfg.HTTPAddr, "environment", cfg.Environment)
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case <-rootCtx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		logger.Info("shutting down")
		return server.Shutdown(shutdownCtx)
	case err := <-serverErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func newLogger(environment string) *slog.Logger {
	options := &slog.HandlerOptions{Level: slog.LevelInfo}
	if environment == "development" {
		return slog.New(slog.NewTextHandler(os.Stdout, options))
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, options))
}
