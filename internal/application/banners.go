package application

import (
	"context"
	"errors"
	"fmt"
	"gachaslop/internal/domain"
	"gachaslop/internal/ports"
	"time"
)

var ErrBannersUnavailable = errors.New("banner source unavailable")

type bannerGate struct {
	lock    chan struct{}
	retryAt time.Time
}
type BannerService struct {
	repo    ports.BannerRepository
	source  ports.BannerSource
	clock   ports.Clock
	ttl     time.Duration
	gates   map[domain.Game]*bannerGate
	artwork ports.BannerArtwork
}

func (s *BannerService) WithArtwork(artwork ports.BannerArtwork) *BannerService {
	s.artwork = artwork
	return s
}

func NewBannerService(repo ports.BannerRepository, source ports.BannerSource, clock ports.Clock, ttl time.Duration) *BannerService {
	gates := make(map[domain.Game]*bannerGate)
	for _, game := range []domain.Game{domain.GameGenshin, domain.GameHSR, domain.GameZZZ, domain.GameWuWa} {
		gates[game] = &bannerGate{lock: make(chan struct{}, 1)}
	}
	return &BannerService{repo: repo, source: source, clock: clock, ttl: ttl, gates: gates}
}

func (s *BannerService) List(ctx context.Context, game domain.Game) (domain.BannerSchedule, error) {
	schedule, err := s.list(ctx, game)
	if err == nil && s.artwork != nil {
		s.artwork.Enrich(ctx, &schedule)
	}
	return schedule, err
}

func (s *BannerService) list(ctx context.Context, game domain.Game) (domain.BannerSchedule, error) {
	gate, ok := s.gates[game]
	if !ok {
		return domain.BannerSchedule{}, domain.ErrInvalidInput
	}
	select {
	case gate.lock <- struct{}{}:
	case <-ctx.Done():
		return domain.BannerSchedule{}, ctx.Err()
	}
	defer func() { <-gate.lock }()
	now := s.clock.Now()
	cached, err := s.repo.GetBannerSchedule(ctx, game)
	hasCache := err == nil
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return domain.BannerSchedule{}, err
	}
	if hasCache && now.Sub(cached.FetchedAt) < s.ttl {
		cached.UpdateStatuses(now)
		return cached, nil
	}
	if !now.Before(gate.retryAt) {
		fetchCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		fresh, fetchErr := s.source.FetchBanners(fetchCtx, game)
		cancel()
		if fetchErr == nil {
			fresh.Game, fresh.FetchedAt = game, s.clock.Now()
			fresh.Stale = false
			if fresh.Items == nil {
				fresh.Items = []domain.Banner{}
			}
			if err := s.repo.SaveBannerSchedule(ctx, fresh); err != nil {
				return domain.BannerSchedule{}, err
			}
			gate.retryAt = time.Time{}
			fresh.UpdateStatuses(s.clock.Now())
			return fresh, nil
		}
		if ctx.Err() != nil {
			return domain.BannerSchedule{}, ctx.Err()
		}
		gate.retryAt = s.clock.Now().Add(time.Minute)
	}
	if hasCache {
		cached.Stale = true
		cached.Warning = "Источник временно недоступен. Показано последнее сохранённое расписание."
		cached.UpdateStatuses(s.clock.Now())
		return cached, nil
	}
	return domain.BannerSchedule{}, fmt.Errorf("%w: %s", ErrBannersUnavailable, game)
}

// Refresh in the application process, not a separate user-facing automation.
func (s *BannerService) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			for _, game := range []domain.Game{domain.GameGenshin, domain.GameHSR, domain.GameZZZ, domain.GameWuWa} {
				if ctx.Err() != nil {
					return
				}
				_, _ = s.List(ctx, game)
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}
