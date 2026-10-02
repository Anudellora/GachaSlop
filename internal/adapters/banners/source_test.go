package banners

import (
	"context"
	"fmt"
	"gachaslop/internal/domain"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHoYoNormalization(t *testing.T) {
	for _, tt := range []struct {
		game   domain.Game
		raw    string
		want   int
		family domain.PoolFamily
	}{
		{domain.GameGenshin, `{"banners":[{"id":1,"name":"Wish","characters":[{"id":1,"name":"Hero","rarity":5,"element":"Anemo","icon":"https://act-webstatic.hoyoverse.com/a.png"}],"start_time":1790000000,"end_time":1791000000}]}`, 5, domain.PoolLimitedCharacter},
		{domain.GameHSR, `{"banners":[{"id":2,"name":"","light_cones":[{"id":2,"name":"Cone","rarity":5}],"start_time":1790000000,"end_time":1791000000}]}`, 5, domain.PoolLimitedWeapon},
		{domain.GameZZZ, `{"banners":[{"banner_type":"GACHA_TYPE_CHARACTER_UP","agents":[{"id":3,"name":"Agent","rarity":"S"},{"id":4,"name":"Agent A","rarity":"A"}],"start_time":null,"end_time":null}]}`, 5, domain.PoolLimitedCharacter},
	} {
		t.Run(string(tt.game), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, tt.raw) }))
			defer server.Close()
			s := New(server.Client(), server.URL, server.URL)
			got, err := s.FetchBanners(context.Background(), tt.game)
			if err != nil {
				t.Fatal(err)
			}
			if len(got.Items) != 1 || got.Items[0].Items[0].Rarity != tt.want || got.Items[0].PoolFamily != tt.family || got.Items[0].Title == "" || got.Items[0].ID == "" {
				t.Fatalf("unexpected: %#v", got)
			}
		})
	}
}

func TestRejectBrokenSource(t *testing.T) {
	for _, raw := range []string{`{}`, `{"banners":null}`, `{"banners":"wrong"}`, `{"banners":[{}]}`} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, raw) }))
		_, err := New(server.Client(), server.URL, server.URL).FetchBanners(context.Background(), domain.GameGenshin)
		server.Close()
		if err == nil {
			t.Fatalf("accepted broken schema: %s", raw)
		}
	}
}

func TestWuWaUsesDurationNotAnnouncementWindow(t *testing.T) {
	raw := `{"game":[{"content":"Version 3.7 Update <strong>Maintenance Time: 2026-09-30 04:00 - 2026-09-30 11:00 (UTC+8)</strong>"}],"recommend":[{"id":"10","tabTitle":"[Moon] Featured Resonator Convene","startTimeMs":1,"endTimeMs":2,"tabBanner":"https://aki-gm-resources-back.aki-game.com/notice/image/test.jpg","content":"<div>5-Star Resonator: Hero, 4-Star Resonators: A, B, and C receive boosted drop rates!</div><div>✦Duration✦</div><div>Version 3.7 update - 2026-10-22 09:59 (server time)</div><div>✦Eligibility✦</div>"},{"id":"11","tabTitle":"[Sword] Featured Weapon Convene","content":"✦Duration✦ 2026-10-22 10:00 - 2026-11-11 11:59 (server time) ✦Eligibility✦"}]}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, raw) }))
	defer server.Close()
	got, err := New(server.Client(), server.URL, server.URL).FetchBanners(context.Background(), domain.GameWuWa)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Items) != 2 {
		t.Fatalf("%#v", got)
	}
	b := got.Items[0]
	if b.StartsAt == nil || b.StartsAt.Format("2006-01-02T15:04Z") != "2026-09-30T03:00Z" || b.EndsAt.Format("2006-01-02T15:04Z") != "2026-10-22T01:59Z" {
		t.Fatalf("wrong dates: %#v", b)
	}
	if len(b.Items) != 4 || b.Items[0].Name != "Hero" {
		t.Fatalf("wrong items: %#v", b.Items)
	}
	if got.Items[1].StartsAt == nil || got.Items[1].PoolFamily != domain.PoolLimitedWeapon {
		t.Fatalf("wrong future weapon: %#v", got.Items[1])
	}
}

func TestWuWaUnknownDatesAndImageAllowlist(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"game":[],"activity":[{"id":"1","tabTitle":"[Soon] Featured Resonator Convene","content":"Dates to be announced","startTimeMs":1,"endTimeMs":2,"tabBanner":"https://evil.example/img"}]}`)
	}))
	defer server.Close()
	got, err := New(server.Client(), server.URL, server.URL).FetchBanners(context.Background(), domain.GameWuWa)
	if err != nil {
		t.Fatal(err)
	}
	if got.Items[0].StartsAt != nil || got.Items[0].EndsAt != nil || got.Items[0].ImageURL != "" || got.Warning == "" {
		t.Fatalf("%#v", got)
	}
}
