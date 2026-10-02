package hoyo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"gachaslop/internal/domain"
)

const pageSize = 20

var defaultEndpoints = map[domain.Game]string{
	domain.GameGenshin: "https://public-operation-hk4e-sg.hoyoverse.com/gacha_info/api/getGachaLog",
	domain.GameHSR:     "https://public-operation-hkrpg-sg.hoyoverse.com/common/gacha_record/api/getGachaLog",
	domain.GameZZZ:     "https://public-operation-nap-sg.hoyoverse.com/common/gacha_record/api/getGachaLog",
}

var sourcePools = map[domain.Game][]string{
	domain.GameGenshin: {"100", "200", "301", "302", "500"},
	domain.GameHSR:     {"1", "2", "11", "12", "21", "22"},
	domain.GameZZZ:     {"1", "2", "3", "5"},
}

var allowedSourceHosts = map[string]struct{}{
	"webstatic-sea.hoyoverse.com":             {},
	"webstatic-sea.mihoyo.com":                {},
	"public-operation-hk4e-sg.hoyoverse.com":  {},
	"public-operation-hkrpg-sg.hoyoverse.com": {},
	"public-operation-nap-sg.hoyoverse.com":   {},
	"public-operation-hk4e.mihoyo.com":        {},
	"api-takumi.mihoyo.com":                   {},
}

type Client struct {
	game     domain.Game
	http     *http.Client
	endpoint string
	delay    time.Duration
	maxPages int
}

type Options struct {
	Endpoint string
	Delay    time.Duration
	MaxPages int
}

func New(game domain.Game, httpClient *http.Client, options Options) (*Client, error) {
	endpoint, ok := defaultEndpoints[game]
	if !ok {
		return nil, fmt.Errorf("hoyo client does not support %s", game)
	}
	if options.Endpoint != "" {
		endpoint = options.Endpoint
	}
	if options.MaxPages <= 0 {
		options.MaxPages = 5000
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 20 * time.Second}
	}
	return &Client{game: game, http: httpClient, endpoint: endpoint, delay: options.Delay, maxPages: options.MaxPages}, nil
}

func (c *Client) Fetch(ctx context.Context, rawURL string, stopIDs map[string]string) (domain.ImportBatch, error) {
	params, err := parseSourceURL(rawURL)
	if err != nil {
		return domain.ImportBatch{}, fmt.Errorf("%w: %v", domain.ErrInvalidInput, err)
	}
	if strings.HasSuffix(params.Get("game_biz"), "_cn") {
		return domain.ImportBatch{}, fmt.Errorf("%w: China servers are not enabled in this build", domain.ErrInvalidInput)
	}

	batch := domain.ImportBatch{Account: domain.Account{
		Game:     c.game,
		Region:   params.Get("region"),
		Language: params.Get("lang"),
	}}
	for _, sourcePool := range sourcePools[c.game] {
		items, metadata, err := c.fetchPool(ctx, params, sourcePool, stopIDs[sourcePool])
		if err != nil {
			return domain.ImportBatch{}, err
		}
		if batch.Account.UID == "" && metadata.UID != "" {
			batch.Account.UID = metadata.UID
		}
		if metadata.UID != "" && batch.Account.UID != metadata.UID {
			return domain.ImportBatch{}, fmt.Errorf("%w: upstream returned several UIDs", domain.ErrUpstream)
		}
		if metadata.Region != "" {
			batch.Account.Region = metadata.Region
		}
		if metadata.TimezoneOffset != 0 {
			batch.Account.TimezoneOffset = metadata.TimezoneOffset
		}
		batch.Pulls = append(batch.Pulls, items...)
	}
	if batch.Account.UID == "" && len(stopIDs) == 0 {
		return domain.ImportBatch{}, fmt.Errorf("%w: no pull records were returned; UID cannot be determined", domain.ErrInvalidInput)
	}
	return batch, nil
}

type poolMetadata struct {
	UID            string
	Region         string
	TimezoneOffset int
}

func (c *Client) fetchPool(ctx context.Context, base url.Values, sourcePool, stopID string) ([]domain.Pull, poolMetadata, error) {
	params := cloneValues(base)
	params.Set("gacha_type", sourcePool)
	params.Set("size", strconv.Itoa(pageSize))
	params.Set("page", "1")
	params.Set("end_id", "0")

	items := make([]domain.Pull, 0)
	metadata := poolMetadata{}
	for page := 1; page <= c.maxPages; page++ {
		params.Set("page", strconv.Itoa(page))
		requestURL := c.endpoint + "?" + params.Encode()
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
		if err != nil {
			return nil, metadata, fmt.Errorf("build hoyo request: %w", err)
		}
		request.Header.Set("Accept", "application/json")
		request.Header.Set("User-Agent", "GachaSLop/0.1")

		response, err := c.http.Do(request)
		if err != nil {
			return nil, metadata, fmt.Errorf("%w: pull history network request failed", domain.ErrUpstream)
		}
		var payload apiResponse
		decodeErr := json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(&payload)
		_ = response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return nil, metadata, fmt.Errorf("%w: pull history returned HTTP %d", domain.ErrUpstream, response.StatusCode)
		}
		if decodeErr != nil {
			return nil, metadata, fmt.Errorf("%w: decode pull history: %v", domain.ErrUpstream, decodeErr)
		}
		if payload.Retcode != 0 {
			if payload.Retcode == -100 || payload.Retcode == -101 {
				return nil, metadata, fmt.Errorf("%w: hoyo retcode %d", domain.ErrSourceExpired, payload.Retcode)
			}
			return nil, metadata, fmt.Errorf("%w: hoyo retcode %d: %s", domain.ErrUpstream, payload.Retcode, safeMessage(payload.Message))
		}
		if payload.Data == nil || len(payload.Data.List) == 0 {
			break
		}

		metadata.Region = payload.Data.Region
		metadata.TimezoneOffset = payload.Data.RegionTimeZone
		stopped := false
		for _, item := range payload.Data.List {
			if metadata.UID == "" {
				metadata.UID = item.UID
			}
			if stopID != "" && item.ID == stopID {
				stopped = true
				break
			}
			rarity, err := strconv.Atoi(item.RankType)
			if err != nil {
				return nil, metadata, fmt.Errorf("%w: invalid rarity from hoyo", domain.ErrUpstream)
			}
			items = append(items, domain.Pull{
				ExternalID: item.ID,
				GachaID:    item.GachaID,
				GachaType:  item.GachaType,
				SourcePool: sourcePool,
				PoolFamily: poolFamily(c.game, item.GachaType),
				PityGroup:  pityGroup(c.game, item.GachaType),
				ItemID:     item.ItemID,
				Name:       item.Name,
				ItemType:   item.ItemType,
				Rarity:     rarity,
				PulledAt:   item.Time,
			})
		}
		if stopped || len(payload.Data.List) < pageSize {
			break
		}
		if page == c.maxPages {
			return nil, metadata, fmt.Errorf("%w: pull history exceeded the safety page limit", domain.ErrUpstream)
		}
		params.Set("end_id", payload.Data.List[len(payload.Data.List)-1].ID)
		if err := wait(ctx, c.delay); err != nil {
			return nil, metadata, err
		}
	}
	return items, metadata, nil
}

type apiResponse struct {
	Retcode int      `json:"retcode"`
	Message string   `json:"message"`
	Data    *apiData `json:"data"`
}

type apiData struct {
	List           []apiPull `json:"list"`
	Region         string    `json:"region"`
	RegionTimeZone int       `json:"region_time_zone"`
}

type apiPull struct {
	UID       string `json:"uid"`
	GachaID   string `json:"gacha_id"`
	GachaType string `json:"gacha_type"`
	ItemID    string `json:"item_id"`
	Time      string `json:"time"`
	Name      string `json:"name"`
	ItemType  string `json:"item_type"`
	RankType  string `json:"rank_type"`
	ID        string `json:"id"`
}

func parseSourceURL(raw string) (url.Values, error) {
	if len(raw) == 0 || len(raw) > 32*1024 {
		return nil, errors.New("source_url is empty or too long")
	}
	parsed, err := url.Parse(strings.ReplaceAll(raw, "&amp;", "&"))
	if err != nil || parsed.Scheme != "https" {
		return nil, errors.New("source_url must be a valid HTTPS URL")
	}
	if _, ok := allowedSourceHosts[strings.ToLower(parsed.Hostname())]; !ok {
		return nil, errors.New("source_url host is not an allowed HoYoverse host")
	}
	params := parsed.Query()
	if index := strings.Index(parsed.Fragment, "?"); index >= 0 {
		fragmentParams, err := url.ParseQuery(parsed.Fragment[index+1:])
		if err != nil {
			return nil, errors.New("invalid source_url fragment")
		}
		for key, values := range fragmentParams {
			for _, value := range values {
				params.Add(key, value)
			}
		}
	}
	if params.Get("authkey") == "" {
		return nil, errors.New("source_url does not contain authkey")
	}
	if params.Get("lang") == "" {
		params.Set("lang", "en-us")
	}
	return params, nil
}

func poolFamily(game domain.Game, gachaType string) domain.PoolFamily {
	switch game {
	case domain.GameGenshin:
		switch gachaType {
		case "100":
			return domain.PoolBeginner
		case "200":
			return domain.PoolStandard
		case "301", "400":
			return domain.PoolLimitedCharacter
		case "302":
			return domain.PoolLimitedWeapon
		case "500":
			return domain.PoolChronicled
		}
	case domain.GameHSR:
		switch gachaType {
		case "1":
			return domain.PoolStandard
		case "2":
			return domain.PoolBeginner
		case "11", "21":
			return domain.PoolLimitedCharacter
		case "12", "22":
			return domain.PoolLimitedWeapon
		}
	case domain.GameZZZ:
		switch gachaType {
		case "1":
			return domain.PoolStandard
		case "2":
			return domain.PoolLimitedCharacter
		case "3":
			return domain.PoolLimitedWeapon
		case "5":
			return domain.PoolBangboo
		}
	}
	return domain.PoolOther
}

func pityGroup(game domain.Game, gachaType string) string {
	canonical := gachaType
	if game == domain.GameGenshin && gachaType == "400" {
		canonical = "301"
	}
	return string(game) + ":" + canonical
}

func cloneValues(input url.Values) url.Values {
	result := make(url.Values, len(input))
	for key, values := range input {
		result[key] = append([]string(nil), values...)
	}
	return result
}

func wait(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func safeMessage(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 200 {
		return value[:200]
	}
	return value
}
