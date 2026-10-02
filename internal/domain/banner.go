package domain

import "time"

type BannerItem struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	ImageURL string `json:"image_url,omitempty"`
	ArtURL   string `json:"art_url,omitempty"`
	Rarity   int    `json:"rarity"`
	Element  string `json:"element,omitempty"`
	Kind     string `json:"kind"`
}

type Banner struct {
	ID         string       `json:"id"`
	Title      string       `json:"title"`
	Version    string       `json:"version,omitempty"`
	PoolFamily PoolFamily   `json:"pool_family"`
	ImageURL   string       `json:"image_url,omitempty"`
	ImageKind  string       `json:"image_kind"`
	Items      []BannerItem `json:"items"`
	StartsAt   *time.Time   `json:"starts_at"`
	EndsAt     *time.Time   `json:"ends_at"`
	Status     string       `json:"status"`
}

type BannerSchedule struct {
	Game      Game      `json:"game"`
	Items     []Banner  `json:"items"`
	Source    string    `json:"source"`
	SourceURL string    `json:"source_url"`
	TimeNote  string    `json:"time_note"`
	FetchedAt time.Time `json:"fetched_at"`
	Stale     bool      `json:"stale"`
	Warning   string    `json:"warning,omitempty"`
}

func (s *BannerSchedule) UpdateStatuses(now time.Time) {
	for i := range s.Items {
		b := &s.Items[i]
		switch {
		case b.EndsAt != nil && !now.Before(*b.EndsAt):
			b.Status = "ended"
		case b.StartsAt != nil && now.Before(*b.StartsAt):
			b.Status = "upcoming"
		case b.StartsAt != nil && b.EndsAt != nil:
			b.Status = "current"
		default:
			b.Status = "unknown"
		}
	}
}
