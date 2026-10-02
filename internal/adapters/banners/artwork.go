package banners

import (
	"context"
	"gachaslop/internal/domain"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const ArtworkCatalogBase = "https://raw.githubusercontent.com/EnkaNetwork/API-docs/master/store"

type artCatalog struct {
	gate      chan struct{}
	urls      map[string]string
	nextCheck time.Time
}

// Artwork reads only the three fixed public character catalogs. A catalog outage
// must never turn an otherwise valid schedule into an error.
type Artwork struct {
	source   *Source
	base     string
	catalogs map[domain.Game]*artCatalog
}

func NewArtwork(client *http.Client, base string) *Artwork {
	a := &Artwork{source: New(client, "", ""), base: strings.TrimRight(base, "/"), catalogs: map[domain.Game]*artCatalog{}}
	for _, game := range []domain.Game{domain.GameGenshin, domain.GameHSR, domain.GameZZZ} {
		a.catalogs[game] = &artCatalog{gate: make(chan struct{}, 1)}
	}
	return a
}

func (a *Artwork) Enrich(ctx context.Context, schedule *domain.BannerSchedule) {
	c, ok := a.catalogs[schedule.Game]
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
	defer cancel()
	select {
	case c.gate <- struct{}{}:
	case <-ctx.Done():
		return
	}
	defer func() { <-c.gate }()
	if !time.Now().Before(c.nextCheck) {
		slug := map[domain.Game]string{domain.GameGenshin: "gi", domain.GameHSR: "hsr", domain.GameZZZ: "zzz"}[schedule.Game]
		var entries map[string]struct {
			Side  string `json:"SideIconName"`
			Cutin string `json:"AvatarCutinFrontImgPath"`
			Image string `json:"Image"`
		}
		if err := a.source.get(ctx, a.base+"/"+slug+"/avatars.json", &entries); err == nil && len(entries) > 0 {
			urls := map[string]string{}
			for id, entry := range entries {
				if url := characterArt(schedule.Game, entry.Side, entry.Cutin, entry.Image); url != "" {
					urls[id] = url
				}
			}
			if len(urls) > 0 {
				c.urls, c.nextCheck = urls, time.Now().Add(6*time.Hour)
			} else {
				c.nextCheck = time.Now().Add(time.Minute)
			}
		} else {
			c.nextCheck = time.Now().Add(time.Minute)
		}
	}
	schedule.Items = append([]domain.Banner{}, schedule.Items...)
	for i := range schedule.Items {
		// Copy before decorating: repositories may return shared immutable snapshots.
		items := append([]domain.BannerItem{}, schedule.Items[i].Items...)
		for j := range items {
			if items[j].Kind == "character" {
				items[j].ArtURL = c.urls[items[j].ID]
			}
		}
		schedule.Items[i].Items = items
	}
}

var giArt = regexp.MustCompile(`^/ui/UI_AvatarIcon_Side_([A-Za-z0-9_]+)\.png$`)
var hsrArt = regexp.MustCompile(`^/ui/hsr/SpriteOutput/AvatarDrawCard/[0-9]+\.png$`)
var zzzArt = regexp.MustCompile(`^/ui/zzz/IconRole([0-9]+)\.png$`)

func characterArt(game domain.Game, side, cutin, image string) string {
	switch game {
	case domain.GameGenshin:
		if m := giArt.FindStringSubmatch(side); m != nil {
			return "https://enka.network/ui/UI_Gacha_AvatarImg_" + m[1] + ".png"
		}
	case domain.GameHSR:
		if hsrArt.MatchString(cutin) {
			return "https://enka.network" + cutin
		}
	case domain.GameZZZ:
		if m := zzzArt.FindStringSubmatch(image); m != nil {
			return "https://enka.network/ui/zzz/IconRole" + m[1] + ".png"
		}
	}
	return ""
}
