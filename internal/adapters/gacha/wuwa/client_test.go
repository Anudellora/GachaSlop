package wuwa

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"gachaslop/internal/domain"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return fn(request) }

func TestFetchNormalizesWuWaPulls(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(request.Body)
		payload := `{"code":0,"message":"OK","data":[]}`
		if strings.Contains(string(body), `"cardPoolType":1`) {
			payload = `{"code":0,"message":"OK","data":[{"name":"Test Resonator","time":"2026-09-14 12:00:00","qualityLevel":5,"resourceType":"Resonator","resourceId":"9001","cardPoolType":"1","cardPoolId":"777"}]}`
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(payload)),
		}, nil
	})}
	client := New(httpClient, 0)
	sourceURL := "https://aki-gm-resources-oversea.aki-game.net/aki/gacha/index.html#/record?svr_id=6&player_id=123456&record_id=secret&resources_id=9&lang=en"

	batch, err := client.Fetch(context.Background(), sourceURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if batch.Account.UID != "123456" || batch.Account.Region != "6" {
		t.Fatalf("unexpected account: %#v", batch.Account)
	}
	if len(batch.Pulls) != 1 {
		t.Fatalf("expected one pull, got %d", len(batch.Pulls))
	}
	pull := batch.Pulls[0]
	if pull.PoolFamily != domain.PoolLimitedCharacter || pull.PityGroup != "wuwa:1" || !strings.HasPrefix(pull.ExternalID, "wuwa_") {
		t.Fatalf("unexpected pull: %#v", pull)
	}

	repeat, err := client.Fetch(context.Background(), sourceURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if repeat.Pulls[0].ExternalID != pull.ExternalID {
		t.Fatal("synthetic pull ID must be stable")
	}
}

func TestParseSourceURLReadsFragmentQuery(t *testing.T) {
	params, err := parseSourceURL("https://aki-gm-resources.aki-game.net/aki/gacha/index.html#/record?svr_id=1&player_id=2&record_id=3")
	if err != nil {
		t.Fatal(err)
	}
	if params.Endpoint != "https://gmserver-api.aki-game2.net/gacha/record/query" || params.PlayerID != "2" {
		t.Fatalf("unexpected params: %#v", params)
	}
}
