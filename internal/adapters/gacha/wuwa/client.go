package wuwa

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
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

var allowedHosts = map[string]struct{}{
	"aki-gm-resources.aki-game.net":         {},
	"aki-gm-resources-oversea.aki-game.net": {},
	"aki-gm-resources.aki-game.com":         {},
	"aki-gm-resources-oversea.aki-game.com": {},
}

var pools = []string{"1", "2", "3", "4", "5", "6", "7"}

type Client struct {
	http  *http.Client
	delay time.Duration
}

func New(httpClient *http.Client, delay time.Duration) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 20 * time.Second}
	}
	return &Client{http: httpClient, delay: delay}
}

func (c *Client) Fetch(ctx context.Context, rawURL string, _ map[string]string) (domain.ImportBatch, error) {
	params, err := parseSourceURL(rawURL)
	if err != nil {
		return domain.ImportBatch{}, fmt.Errorf("%w: %v", domain.ErrInvalidInput, err)
	}
	batch := domain.ImportBatch{Account: domain.Account{
		Game:     domain.GameWuWa,
		UID:      params.PlayerID,
		Region:   params.ServerID,
		Language: params.LanguageCode,
	}}
	for index, sourcePool := range pools {
		items, err := c.fetchPool(ctx, params, sourcePool)
		if err != nil {
			return domain.ImportBatch{}, err
		}
		batch.Pulls = append(batch.Pulls, items...)
		if index < len(pools)-1 {
			if err := wait(ctx, c.delay); err != nil {
				return domain.ImportBatch{}, err
			}
		}
	}
	return batch, nil
}

type sourceParams struct {
	Endpoint     string
	ServerID     string
	PlayerID     string
	RecordID     string
	LanguageCode string
	CardPoolID   string
}

func parseSourceURL(raw string) (sourceParams, error) {
	if len(raw) == 0 || len(raw) > 32*1024 {
		return sourceParams{}, errors.New("source_url is empty or too long")
	}
	parsed, err := url.Parse(strings.ReplaceAll(raw, "&amp;", "&"))
	if err != nil || parsed.Scheme != "https" {
		return sourceParams{}, errors.New("source_url must be a valid HTTPS URL")
	}
	host := strings.ToLower(parsed.Hostname())
	if _, ok := allowedHosts[host]; !ok {
		return sourceParams{}, errors.New("source_url host is not an allowed Kuro host")
	}
	queryText := parsed.RawQuery
	if index := strings.Index(parsed.Fragment, "?"); index >= 0 {
		queryText = parsed.Fragment[index+1:]
	}
	query, err := url.ParseQuery(queryText)
	if err != nil {
		return sourceParams{}, errors.New("invalid source_url parameters")
	}
	endpoint := "https://gmserver-api.aki-game2.net/gacha/record/query"
	if strings.HasSuffix(host, ".com") {
		endpoint = "https://gmserver-api.aki-game2.com/gacha/record/query"
	}
	params := sourceParams{
		Endpoint:     endpoint,
		ServerID:     query.Get("svr_id"),
		PlayerID:     query.Get("player_id"),
		RecordID:     query.Get("record_id"),
		LanguageCode: query.Get("lang"),
		CardPoolID:   query.Get("resources_id"),
	}
	if params.LanguageCode == "" {
		params.LanguageCode = "en"
	}
	if params.ServerID == "" || params.PlayerID == "" || params.RecordID == "" {
		return sourceParams{}, errors.New("source_url is missing svr_id, player_id or record_id")
	}
	return params, nil
}

func (c *Client) fetchPool(ctx context.Context, params sourceParams, sourcePool string) ([]domain.Pull, error) {
	poolType, _ := strconv.Atoi(sourcePool)
	body, err := json.Marshal(map[string]any{
		"cardPoolId":   params.CardPoolID,
		"cardPoolType": poolType,
		"languageCode": params.LanguageCode,
		"playerId":     params.PlayerID,
		"recordId":     params.RecordID,
		"serverId":     params.ServerID,
	})
	if err != nil {
		return nil, fmt.Errorf("encode Kuro request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, params.Endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build Kuro request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "GachaSLop/0.1")

	response, err := c.http.Do(request)
	if err != nil {
		return nil, fmt.Errorf("%w: pull history network request failed", domain.ErrUpstream)
	}
	var payload apiResponse
	decodeErr := json.NewDecoder(io.LimitReader(response.Body, 16<<20)).Decode(&payload)
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: pull history returned HTTP %d", domain.ErrUpstream, response.StatusCode)
	}
	if decodeErr != nil {
		return nil, fmt.Errorf("%w: decode pull history: %v", domain.ErrUpstream, decodeErr)
	}
	if payload.Code != 0 {
		return nil, fmt.Errorf("%w: Kuro code %d: %s", domain.ErrUpstream, payload.Code, safeMessage(payload.Message))
	}

	externalIDs := make([]string, len(payload.Data))
	occurrences := make(map[string]int)
	for index := len(payload.Data) - 1; index >= 0; index-- {
		item := payload.Data[index]
		base := strings.Join([]string{sourcePool, item.Time, string(item.ResourceID), item.Name, strconv.Itoa(int(item.QualityLevel)), item.ResourceType}, "|")
		occurrences[base]++
		externalIDs[index] = syntheticID(params.PlayerID, base, occurrences[base])
	}
	items := make([]domain.Pull, 0, len(payload.Data))
	for index, item := range payload.Data {
		externalID := externalIDs[index]
		actualPool := string(item.CardPoolType)
		if actualPool == "" {
			actualPool = sourcePool
		}
		items = append(items, domain.Pull{
			ExternalID: externalID,
			GachaID:    string(item.CardPoolID),
			GachaType:  actualPool,
			SourcePool: sourcePool,
			PoolFamily: poolFamily(actualPool),
			PityGroup:  "wuwa:" + actualPool,
			ItemID:     string(item.ResourceID),
			Name:       item.Name,
			ItemType:   item.ResourceType,
			Rarity:     int(item.QualityLevel),
			PulledAt:   item.Time,
		})
	}
	return items, nil
}

type apiResponse struct {
	Code    int       `json:"code"`
	Message string    `json:"message"`
	Data    []apiPull `json:"data"`
}

type apiPull struct {
	Name         string     `json:"name"`
	Time         string     `json:"time"`
	QualityLevel flexInt    `json:"qualityLevel"`
	ResourceType string     `json:"resourceType"`
	ResourceID   flexString `json:"resourceId"`
	CardPoolType flexString `json:"cardPoolType"`
	CardPoolID   flexString `json:"cardPoolId"`
}

type flexInt int

func (value *flexInt) UnmarshalJSON(body []byte) error {
	raw := strings.Trim(string(body), `"`)
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return err
	}
	*value = flexInt(parsed)
	return nil
}

type flexString string

func (value *flexString) UnmarshalJSON(body []byte) error {
	if string(body) == "null" {
		*value = ""
		return nil
	}
	if len(body) > 0 && body[0] == '"' {
		var parsed string
		if err := json.Unmarshal(body, &parsed); err != nil {
			return err
		}
		*value = flexString(parsed)
		return nil
	}
	*value = flexString(string(body))
	return nil
}

func syntheticID(playerID, base string, occurrence int) string {
	digest := sha256.Sum256([]byte(playerID + "|" + base + "|" + strconv.Itoa(occurrence)))
	return "wuwa_" + hex.EncodeToString(digest[:16])
}

func poolFamily(poolType string) domain.PoolFamily {
	switch poolType {
	case "1":
		return domain.PoolLimitedCharacter
	case "2":
		return domain.PoolLimitedWeapon
	case "3", "4":
		return domain.PoolStandard
	case "5", "6", "7":
		return domain.PoolBeginner
	default:
		return domain.PoolOther
	}
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
