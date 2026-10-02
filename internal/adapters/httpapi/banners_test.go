package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"gachaslop/internal/adapters/storage/sqlite"
	"gachaslop/internal/application"
	"gachaslop/internal/domain"
	platform "gachaslop/internal/platform/id"
)

type scheduleSource struct{ fail bool }

func (s scheduleSource) FetchBanners(_ context.Context, game domain.Game) (domain.BannerSchedule, error) {
	if s.fail {
		return domain.BannerSchedule{}, errors.New("offline")
	}
	start, end := time.Now().Add(-time.Hour), time.Now().Add(time.Hour)
	return domain.BannerSchedule{Game: game, Source: "test", Items: []domain.Banner{{
		ID: "live", Title: "Test banner", StartsAt: &start, EndsAt: &end,
		Items: []domain.BannerItem{}, ImageKind: "icon", PoolFamily: domain.PoolLimitedCharacter,
	}}}, nil
}

func TestBannerHTTPAndPersistentFallback(t *testing.T) {
	ctx := context.Background()
	dsn := "file:" + filepath.Join(t.TempDir(), "banners.db")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	var fetchedAt time.Time
	for _, offline := range []bool{false, true} {
		store, err := sqlite.Open(ctx, dsn, 1)
		if err != nil {
			t.Fatal(err)
		}
		// Reopen the actual database and force expiry to simulate a restart
		// followed by an unavailable source, without any in-memory snapshot.
		service := application.NewBannerService(store, scheduleSource{fail: offline}, platform.Clock{}, 0)
		handler := New(nil, store, logger, platform.Generator{}).WithBanners(service).Handler()
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/banners?game=genshin", nil))
		if w.Code != 200 {
			t.Fatalf("got %d: %s", w.Code, w.Body)
		}
		var result domain.BannerSchedule
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if result.Game != domain.GameGenshin || result.Stale != offline || len(result.Items) != 1 || result.Items[0].Status != "current" {
			t.Fatalf("unexpected schedule: %#v", result)
		}
		if offline && (result.Warning == "" || !result.FetchedAt.Equal(fetchedAt)) {
			t.Fatalf("stale metadata lost: %#v", result)
		}
		fetchedAt = result.FetchedAt
		if offline {
			for _, tt := range []struct {
				query string
				code  int
			}{{"", 400}, {"?game=invalid", 400}, {"?game=wuwa", 503}} {
				w := httptest.NewRecorder()
				handler.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/banners"+tt.query, nil))
				if w.Code != tt.code || (tt.code == 503 && w.Header().Get("Retry-After") != "60") {
					t.Fatalf("%s: got %d: %s", tt.query, w.Code, w.Body)
				}
			}
		}
		if err := store.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
