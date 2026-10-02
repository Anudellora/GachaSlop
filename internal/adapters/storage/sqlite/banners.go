package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"gachaslop/internal/domain"
)

func (s *Store) GetBannerSchedule(ctx context.Context, game domain.Game) (domain.BannerSchedule, error) {
	var data []byte
	err := s.db.QueryRowContext(ctx, "SELECT payload FROM banner_schedules WHERE game = ?", game).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.BannerSchedule{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.BannerSchedule{}, err
	}
	var result domain.BannerSchedule
	err = json.Unmarshal(data, &result)
	return result, err
}

func (s *Store) SaveBannerSchedule(ctx context.Context, schedule domain.BannerSchedule) error {
	data, err := json.Marshal(schedule)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO banner_schedules(game, payload, fetched_at) VALUES (?, ?, ?)
		ON CONFLICT(game) DO UPDATE SET payload = excluded.payload, fetched_at = excluded.fetched_at`,
		schedule.Game, data, formatTime(schedule.FetchedAt))
	return err
}
