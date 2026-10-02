package banners

import (
	"context"
	"fmt"
	"gachaslop/internal/domain"
	"html"
	"regexp"
	"strings"
	"time"
)

var (
	tags          = regexp.MustCompile(`<[^>]*>`)
	versionRE     = regexp.MustCompile(`(?i)Version\s+(\d+\.\d+)`)
	dateRE        = regexp.MustCompile(`\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}`)
	maintenanceRE = regexp.MustCompile(`(?i)Maintenance Time:\s*(\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2})\s*[-–]\s*(\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2})\s*\(UTC\+8\)`)
	durationRE    = regexp.MustCompile(`(?i)Duration[^A-Za-z0-9]*(.+?)(?:✦|Eligibility|Convene Rules|$)`)
	item5RE       = regexp.MustCompile(`(?i)5-Star (?:Resonator|Weapon):\s*(.+?)(?:,\s*4-Star|\s+receive boosted)`)
	item4RE       = regexp.MustCompile(`(?i)4-Star (?:Resonators|Weapons):\s*(.+?)\s+receive boosted`)
)

type notice struct {
	ID      string `json:"id"`
	Title   string `json:"tabTitle"`
	Content string `json:"content"`
	Image   string `json:"tabBanner"`
}

func plain(raw string) string {
	return strings.Join(strings.Fields(html.UnescapeString(tags.ReplaceAllString(raw, " "))), " ")
}

func (s *Source) wuwa(ctx context.Context) (domain.BannerSchedule, error) {
	var response map[string][]notice
	if err := s.get(ctx, s.wuwaURL, &response); err != nil {
		return domain.BannerSchedule{}, err
	}
	if _, ok := response["game"]; !ok {
		return domain.BannerSchedule{}, fmt.Errorf("missing game notices")
	}
	updates := map[string]*time.Time{}
	for _, n := range response["game"] {
		text := plain(n.Content)
		v := versionRE.FindStringSubmatch(text)
		m := maintenanceRE.FindStringSubmatch(text)
		if len(v) > 1 && len(m) > 2 {
			updates[v[1]] = parseAsia(m[2])
		}
	}
	result := domain.BannerSchedule{Game: domain.GameWuWa, Items: []domain.Banner{}, Source: "Kuro Games · объявления", SourceURL: s.wuwaURL,
		TimeNote: "Официальные объявления на английском. Server time интерпретируется для Азии (UTC+8); для других серверов сроки могут отличаться."}
	seen := map[string]bool{}
	for _, section := range []string{"recommend", "activity", "game"} {
		for _, n := range response[section] {
			if seen[n.ID] || !strings.Contains(n.Title, "Featured") || !strings.Contains(n.Title, "Convene") {
				continue
			}
			seen[n.ID] = true
			text := plain(n.Content)
			family, kind := domain.PoolLimitedCharacter, "character"
			if strings.Contains(n.Title, "Weapon") {
				family, kind = domain.PoolLimitedWeapon, "weapon"
			}
			var start, end *time.Time
			version := ""
			if v := versionRE.FindStringSubmatch(text); len(v) > 1 {
				version = v[1]
			}
			if d := durationRE.FindStringSubmatch(text); len(d) > 1 {
				dates := dateRE.FindAllString(d[1], -1)
				if len(dates) == 2 {
					start, end = parseAsia(dates[0]), parseAsia(dates[1])
				} else if len(dates) == 1 {
					end = parseAsia(dates[0])
					if strings.Contains(strings.ToLower(d[1]), "update") {
						start = updates[version]
					}
				}
			}
			// Announcement visibility timestamps are NOT event dates. Unknown dates stay null.
			if start != nil && end != nil && !end.After(*start) {
				return domain.BannerSchedule{}, fmt.Errorf("invalid convene duration")
			}
			if start == nil || end == nil {
				result.Warning = "Не все сроки удалось прочитать из объявления. Неизвестные даты не заменяются предположениями."
			}
			items := []domain.BannerItem{}
			if names := item5RE.FindStringSubmatch(text); len(names) > 1 {
				items = append(items, domain.BannerItem{ID: n.ID + "-5", Name: strings.TrimSpace(names[1]), Rarity: 5, Kind: kind})
			}
			if names := item4RE.FindStringSubmatch(text); len(names) > 1 {
				for i, name := range strings.Split(strings.ReplaceAll(names[1], " and ", ","), ",") {
					name = strings.TrimSpace(name)
					if name != "" {
						items = append(items, domain.BannerItem{ID: fmt.Sprintf("%s-4-%d", n.ID, i), Name: name, Rarity: 4, Kind: kind})
					}
				}
			}
			title := n.Title
			if l, r := strings.Index(title, "["), strings.Index(title, "]"); l >= 0 && r > l {
				title = title[l+1 : r]
			}
			result.Items = append(result.Items, domain.Banner{ID: "wuwa-" + n.ID, Title: title, Version: version, PoolFamily: family,
				Items: items, ImageURL: imageURL(n.Image), ImageKind: "banner", StartsAt: start, EndsAt: end})
		}
	}
	return result, nil
}

func parseAsia(raw string) *time.Time {
	t, err := time.ParseInLocation("2006-01-02 15:04", raw, time.FixedZone("UTC+8", 8*3600))
	if err != nil {
		return nil
	}
	t = t.UTC()
	return &t
}
