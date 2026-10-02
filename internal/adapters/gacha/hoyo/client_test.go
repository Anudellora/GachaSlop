package hoyo

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

func TestFetchNormalizesAndStopsAtKnownPull(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		payload := `{"retcode":0,"message":"OK","data":{"list":[],"region":"os_euro","region_time_zone":1}}`
		if request.URL.Query().Get("gacha_type") == "301" {
			payload = `{"retcode":0,"message":"OK","data":{"region":"os_euro","region_time_zone":1,"list":[{"uid":"700000001","gacha_id":"banner","gacha_type":"400","item_id":"1001","time":"2026-09-14 12:00:00","name":"Test","item_type":"Character","rank_type":"5","id":"12345"}]}}`
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(payload)),
		}, nil
	})}

	client, err := New(domain.GameGenshin, httpClient, Options{Endpoint: "https://upstream.test/gacha"})
	if err != nil {
		t.Fatal(err)
	}
	sourceURL := "https://webstatic-sea.hoyoverse.com/game/index.html?authkey=secret&game_biz=hk4e_global&region=os_euro&lang=ru-ru"
	batch, err := client.Fetch(context.Background(), sourceURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if batch.Account.UID != "700000001" || batch.Account.TimezoneOffset != 1 {
		t.Fatalf("unexpected account: %#v", batch.Account)
	}
	if len(batch.Pulls) != 1 {
		t.Fatalf("expected one pull, got %d", len(batch.Pulls))
	}
	pull := batch.Pulls[0]
	if pull.SourcePool != "301" || pull.GachaType != "400" || pull.PityGroup != "genshin:301" || pull.PoolFamily != domain.PoolLimitedCharacter {
		t.Fatalf("unexpected pull normalization: %#v", pull)
	}

	incremental, err := client.Fetch(context.Background(), sourceURL, map[string]string{"301": "12345"})
	if err != nil {
		t.Fatal(err)
	}
	if len(incremental.Pulls) != 0 || incremental.Account.UID != "700000001" {
		t.Fatalf("incremental sync did not stop correctly: %#v", incremental)
	}
}

func TestParseSourceURLRejectsArbitraryHost(t *testing.T) {
	_, err := parseSourceURL("https://example.com/?authkey=secret")
	if err == nil {
		t.Fatal("expected arbitrary host to be rejected")
	}
}
