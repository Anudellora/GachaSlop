package banners

import (
	"context"
	"gachaslop/internal/domain"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCharacterArtMapping(t *testing.T) {
	for _, tt := range []struct {
		game                     domain.Game
		side, cutin, image, want string
	}{
		{domain.GameGenshin, "/ui/UI_AvatarIcon_Side_Vesna.png", "", "", "https://enka.network/ui/UI_Gacha_AvatarImg_Vesna.png"},
		{domain.GameHSR, "", "/ui/hsr/SpriteOutput/AvatarDrawCard/1505.png", "", "https://enka.network/ui/hsr/SpriteOutput/AvatarDrawCard/1505.png"},
		{domain.GameZZZ, "", "", "/ui/zzz/IconRole68.png", "https://enka.network/ui/zzz/IconRole68.png"},
		{domain.GameZZZ, "", "", "/ui/zzz/IconRoleSelect68.png", ""},
		{domain.GameHSR, "", "https://evil.example/art.png", "", ""},
		{domain.GameGenshin, "/ui/UI_AvatarIcon_Side_../../evil.png", "", "", ""},
	} {
		if got := characterArt(tt.game, tt.side, tt.cutin, tt.image); got != tt.want {
			t.Fatalf("%s: %q, want %q", tt.game, got, tt.want)
		}
	}
}

func TestArtworkCacheAndFailureDoesNotLoseCalendar(t *testing.T) {
	calls, fail := 0, false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/zzz/avatars.json" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if fail {
			w.WriteHeader(503)
			return
		}
		_, _ = w.Write([]byte(`{"1621":{"Image":"/ui/zzz/IconRole68.png"}}`))
	}))
	defer server.Close()
	a := NewArtwork(server.Client(), server.URL)
	original := domain.BannerSchedule{Game: domain.GameZZZ, Items: []domain.Banner{{ID: "banner", Items: []domain.BannerItem{{ID: "1621", Kind: "character"}, {ID: "1621", Kind: "weapon"}}}}}
	for i := 0; i < 4; i++ {
		if i == 2 {
			fail = true
			a.catalogs[domain.GameZZZ].nextCheck = time.Time{}
		}
		schedule := original
		a.Enrich(context.Background(), &schedule)
		if schedule.Items[0].Items[0].ArtURL != "https://enka.network/ui/zzz/IconRole68.png" || schedule.Items[0].Items[1].ArtURL != "" {
			t.Fatalf("unexpected enrichment: %#v", schedule)
		}
		if original.Items[0].Items[0].ArtURL != "" {
			t.Fatal("modified shared snapshot")
		}
	}
	if calls != 2 {
		t.Fatalf("cache/backoff: %d calls", calls)
	}
	cold := NewArtwork(server.Client(), server.URL)
	schedule := original
	cold.Enrich(context.Background(), &schedule)
	if len(schedule.Items) != 1 || schedule.Items[0].Items[0].ArtURL != "" {
		t.Fatal("cold failure corrupted schedule")
	}
}
