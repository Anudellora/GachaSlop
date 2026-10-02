// Package banners translates public calendars into a vendor-independent model.
package banners

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"gachaslop/internal/domain"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const HoYoBase = "https://api.ennead.cc/mihoyo"
const WuWaURL = "https://aki-gm-resources-back.aki-game.com/gamenotice/G152/76402e5b20be2c39f095a152090afddc/en.json"

type Source struct {
	client            *http.Client
	hoyoBase, wuwaURL string
}

func New(client *http.Client, hoyoBase, wuwaURL string) *Source {
	return &Source{client: client, hoyoBase: strings.TrimRight(hoyoBase, "/"), wuwaURL: wuwaURL}
}

func (s *Source) FetchBanners(ctx context.Context, game domain.Game) (domain.BannerSchedule, error) {
	if game == domain.GameWuWa {
		return s.wuwa(ctx)
	}
	slug := map[domain.Game]string{domain.GameGenshin: "genshin", domain.GameHSR: "starrail", domain.GameZZZ: "zenless"}[game]
	if slug == "" {
		return domain.BannerSchedule{}, domain.ErrInvalidInput
	}
	endpoint := s.hoyoBase + "/" + slug + "/calendar?lang=ru-ru"
	var data struct {
		Banners json.RawMessage `json:"banners"`
	}
	if err := s.get(ctx, endpoint, &data); err != nil {
		return domain.BannerSchedule{}, err
	}
	if len(data.Banners) == 0 || string(data.Banners) == "null" {
		return domain.BannerSchedule{}, fmt.Errorf("missing banners in calendar")
	}
	var raw []hoyoBanner
	if err := json.Unmarshal(data.Banners, &raw); err != nil {
		return domain.BannerSchedule{}, fmt.Errorf("decode banners: %w", err)
	}
	result := domain.BannerSchedule{Game: game, Items: []domain.Banner{}, Source: "Ennead · HoYoLAB", SourceURL: endpoint,
		TimeNote: "Сроки переданы Ennead из игрового календаря. Источник не позволяет выбрать сервер; сверяйте время с вашей игрой."}
	for _, r := range raw {
		items := []domain.BannerItem{}
		chars := append(r.Characters, r.Agents...)
		weapons := append(append(r.Weapons, r.LightCones...), r.Engines...)
		family := domain.PoolLimitedCharacter
		if len(chars) == 0 && len(weapons) > 0 {
			family = domain.PoolLimitedWeapon
		}
		for _, x := range chars {
			items = append(items, convertItem(x, "character", game))
		}
		for _, x := range weapons {
			items = append(items, convertItem(x, "weapon", game))
		}
		if len(items) == 0 {
			return domain.BannerSchedule{}, fmt.Errorf("banner has no featured items")
		}
		sort.SliceStable(items, func(i, j int) bool { return items[i].Rarity > items[j].Rarity })
		featured := []string{}
		for _, item := range items {
			if item.Rarity == items[0].Rarity {
				featured = append(featured, item.Name)
			}
		}
		title := strings.TrimSpace(r.Name)
		if title == "" {
			title = strings.Join(featured, " / ")
		}
		start, end := unixDate(r.Start), unixDate(r.End)
		if start != nil && end != nil && !end.After(*start) {
			return domain.BannerSchedule{}, fmt.Errorf("invalid banner interval")
		}
		identity := fmt.Sprintf("%s:%s:%s:%d:%d:%s", game, r.ID, r.Type, r.Start, r.End, strings.Join(featured, ":"))
		hash := sha256.Sum256([]byte(identity))
		result.Items = append(result.Items, domain.Banner{ID: fmt.Sprintf("%s-%x", game, hash[:12]), Title: title, Version: r.Version,
			PoolFamily: family, Items: items, ImageURL: items[0].ImageURL, ImageKind: "icon", StartsAt: start, EndsAt: end})
	}
	return result, nil
}

func (s *Source) get(ctx context.Context, endpoint string, value any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "GachaSLop/0.2 (public banner calendar)")
	response, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("calendar request failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("calendar HTTP %d", response.StatusCode)
	}
	const maxBytes = 4 << 20
	data, err := io.ReadAll(io.LimitReader(response.Body, maxBytes+1))
	if err != nil {
		return err
	}
	if len(data) > maxBytes {
		return fmt.Errorf("calendar response too large")
	}
	return json.Unmarshal(data, value)
}

type hoyoItem struct {
	ID      json.Number     `json:"id"`
	Name    string          `json:"name"`
	Icon    string          `json:"icon"`
	Element string          `json:"element"`
	Rarity  json.RawMessage `json:"rarity"`
}
type hoyoBanner struct {
	ID         json.Number `json:"id"`
	Name       string      `json:"name"`
	Version    string      `json:"version"`
	Type       string      `json:"banner_type"`
	Characters []hoyoItem  `json:"characters"`
	Agents     []hoyoItem  `json:"agents"`
	Weapons    []hoyoItem  `json:"weapons"`
	LightCones []hoyoItem  `json:"light_cones"`
	Engines    []hoyoItem  `json:"w_engines"`
	Start      int64       `json:"start_time"`
	End        int64       `json:"end_time"`
}

func convertItem(x hoyoItem, kind string, game domain.Game) domain.BannerItem {
	raw := strings.Trim(string(x.Rarity), `"`)
	rarity, _ := strconv.Atoi(raw)
	if raw == "S" {
		rarity = 5
	} else if raw == "A" {
		rarity = 4
	} else if raw == "B" {
		rarity = 3
	}
	element := x.Element
	// Numeric HSR category codes are not names; avoid displaying misleading IDs.
	if game == domain.GameHSR {
		if _, err := strconv.Atoi(element); err == nil {
			element = ""
		}
	}
	return domain.BannerItem{ID: x.ID.String(), Name: x.Name, ImageURL: imageURL(x.Icon), Rarity: rarity, Element: element, Kind: kind}
}

func unixDate(value int64) *time.Time {
	if value <= 0 {
		return nil
	}
	t := time.Unix(value, 0).UTC()
	return &t
}

func imageURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil {
		return ""
	}
	switch u.Host {
	case "act-webstatic.hoyoverse.com", "fastcdn.hoyoverse.com", "aki-gm-resources-back.aki-game.com":
		return u.String()
	default:
		return ""
	}
}
