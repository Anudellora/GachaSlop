package application

import (
	"context"
	"errors"
	"gachaslop/internal/domain"
	"sync"
	"testing"
	"time"
)

type bannerMemory struct {
	value domain.BannerSchedule
	set   bool
}

func (m *bannerMemory) GetBannerSchedule(context.Context, domain.Game) (domain.BannerSchedule, error) {
	if !m.set {
		return domain.BannerSchedule{}, domain.ErrNotFound
	}
	copy := m.value
	copy.Items = append([]domain.Banner{}, m.value.Items...)
	return copy, nil
}
func (m *bannerMemory) SaveBannerSchedule(_ context.Context, s domain.BannerSchedule) error {
	m.value = s
	m.set = true
	return nil
}

type bannerClock struct{ now time.Time }

func (c *bannerClock) Now() time.Time { return c.now }

type bannerSource struct {
	calls int
	fail  bool
	now   time.Time
}

func (s *bannerSource) FetchBanners(context.Context, domain.Game) (domain.BannerSchedule, error) {
	s.calls++
	if s.fail {
		return domain.BannerSchedule{}, errors.New("offline")
	}
	start, end := s.now.Add(-time.Hour), s.now.Add(time.Hour)
	return domain.BannerSchedule{Items: []domain.Banner{{ID: "1", StartsAt: &start, EndsAt: &end}}}, nil
}

func TestBannerCacheStaleAndRecovery(t *testing.T) {
	clock := &bannerClock{now: time.Now()}
	source := &bannerSource{now: clock.now}
	repo := &bannerMemory{}
	s := NewBannerService(repo, source, clock, 15*time.Minute)
	first, err := s.List(context.Background(), domain.GameGenshin)
	if err != nil || first.Items[0].Status != "current" {
		t.Fatalf("%#v %v", first, err)
	}
	_, _ = s.List(context.Background(), domain.GameGenshin)
	if source.calls != 1 {
		t.Fatal("cache miss")
	}
	clock.now = clock.now.Add(2 * time.Hour)
	source.fail = true
	stale, err := s.List(context.Background(), domain.GameGenshin)
	if err != nil || !stale.Stale || stale.Items[0].Status != "ended" || !stale.FetchedAt.Equal(first.FetchedAt) {
		t.Fatalf("%#v %v", stale, err)
	}
	_, _ = s.List(context.Background(), domain.GameGenshin)
	if source.calls != 2 {
		t.Fatal("failure backoff missing")
	}
	clock.now = clock.now.Add(time.Minute)
	source.fail = false
	fresh, err := s.List(context.Background(), domain.GameGenshin)
	if err != nil || fresh.Stale || source.calls != 3 {
		t.Fatalf("%#v %v", fresh, err)
	}
}

func TestBannerColdFailureAndCoalescing(t *testing.T) {
	clock := &bannerClock{now: time.Now()}
	source := &bannerSource{now: clock.now, fail: true}
	s := NewBannerService(&bannerMemory{}, source, clock, time.Hour)
	if _, err := s.List(context.Background(), domain.GameWuWa); !errors.Is(err, ErrBannersUnavailable) {
		t.Fatal(err)
	}
	clock.now = clock.now.Add(time.Minute)
	source.fail = false
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.List(context.Background(), domain.GameWuWa); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if source.calls != 2 {
		t.Fatalf("got %d requests", source.calls)
	}
}
