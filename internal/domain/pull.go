package domain

import (
	"sort"
	"time"
)

type PoolFamily string

const (
	PoolBeginner         PoolFamily = "beginner"
	PoolStandard         PoolFamily = "standard"
	PoolLimitedCharacter PoolFamily = "limited_character"
	PoolLimitedWeapon    PoolFamily = "limited_weapon"
	PoolChronicled       PoolFamily = "chronicled"
	PoolBangboo          PoolFamily = "bangboo"
	PoolOther            PoolFamily = "other"
)

type Account struct {
	ID             string    `json:"id"`
	Game           Game      `json:"game"`
	UID            string    `json:"uid"`
	Region         string    `json:"region"`
	Language       string    `json:"language"`
	TimezoneOffset int       `json:"timezone_offset"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type Pull struct {
	ID         string     `json:"id"`
	AccountID  string     `json:"account_id"`
	ExternalID string     `json:"external_id"`
	GachaID    string     `json:"gacha_id,omitempty"`
	GachaType  string     `json:"gacha_type"`
	SourcePool string     `json:"source_pool"`
	PoolFamily PoolFamily `json:"pool_family"`
	PityGroup  string     `json:"pity_group"`
	ItemID     string     `json:"item_id,omitempty"`
	Name       string     `json:"name"`
	ItemType   string     `json:"item_type"`
	Rarity     int        `json:"rarity"`
	PulledAt   string     `json:"pulled_at"`
	CreatedAt  time.Time  `json:"created_at"`
}

type ImportBatch struct {
	Account Account
	Pulls   []Pull
}

type PullFilter struct {
	PoolFamily PoolFamily
	Rarity     int
	Limit      int
	Cursor     string
}

// AccountSummary is a storage-independent read model for the overview screen.
type AccountSummary struct {
	Total         int      `json:"total"`
	FiveStarCount int      `json:"five_star_count"`
	FourStarCount int      `json:"four_star_count"`
	LastImport    *SyncJob `json:"last_import"`
}

type PullPage struct {
	Items      []Pull `json:"items"`
	NextCursor string `json:"next_cursor,omitempty"`
}

type PityCertainty string

const (
	PityExact   PityCertainty = "exact"
	PityAtLeast PityCertainty = "at_least"
)

type GuaranteeState string

const (
	GuaranteeUnknown GuaranteeState = "unknown"
	GuaranteeNo      GuaranteeState = "no"
	GuaranteeYes     GuaranteeState = "yes"
)

type PoolStats struct {
	PityGroup      string         `json:"pity_group"`
	PoolFamily     PoolFamily     `json:"pool_family"`
	Total          int            `json:"total"`
	FiveStarPity   int            `json:"five_star_pity"`
	FiveStarState  PityCertainty  `json:"five_star_certainty"`
	FourStarPity   int            `json:"four_star_pity"`
	FourStarState  PityCertainty  `json:"four_star_certainty"`
	GuaranteeState GuaranteeState `json:"guarantee_state"`
}

func CalculatePity(pulls []Pull) []PoolStats {
	grouped := make(map[string][]Pull)
	for _, pull := range pulls {
		grouped[pull.PityGroup] = append(grouped[pull.PityGroup], pull)
	}
	keys := make([]string, 0, len(grouped))
	for key := range grouped {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	result := make([]PoolStats, 0, len(grouped))
	for _, key := range keys {
		items := grouped[key]
		stats := PoolStats{
			PityGroup:      key,
			PoolFamily:     items[0].PoolFamily,
			Total:          len(items),
			FiveStarState:  PityAtLeast,
			FourStarState:  PityAtLeast,
			GuaranteeState: GuaranteeUnknown,
		}
		for _, pull := range items {
			if stats.FiveStarState != PityExact {
				if pull.Rarity == 5 {
					stats.FiveStarState = PityExact
				} else {
					stats.FiveStarPity++
				}
			}
			if stats.FourStarState != PityExact {
				if pull.Rarity >= 4 {
					stats.FourStarState = PityExact
				} else {
					stats.FourStarPity++
				}
			}
			if stats.FiveStarState == PityExact && stats.FourStarState == PityExact {
				break
			}
		}
		result = append(result, stats)
	}
	return result
}
