package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"gachaslop/internal/adapters/gacha"
	"gachaslop/internal/adapters/queue"
	"gachaslop/internal/adapters/storage/sqlite"
	"gachaslop/internal/application"
	"gachaslop/internal/domain"
	platform "gachaslop/internal/platform/id"
)

type fakeSource struct{}

func (fakeSource) Fetch(context.Context, string, map[string]string) (domain.ImportBatch, error) {
	return domain.ImportBatch{
		Account: domain.Account{Game: domain.GameGenshin, UID: "700000001", Region: "os_euro", Language: "ru-ru"},
		Pulls: []domain.Pull{{
			ExternalID: "100", GachaType: "301", SourcePool: "301",
			PoolFamily: domain.PoolLimitedCharacter, PityGroup: "genshin:301",
			Name: "Test", ItemType: "Character", Rarity: 5, PulledAt: "2026-09-14 12:00:00",
		}},
	}, nil
}

func TestSyncJobHTTPFlow(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	store, err := sqlite.Open(ctx, "file:"+filepath.Join(t.TempDir(), "api.db"), 2)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ids := platform.Generator{}
	clock := platform.Clock{}
	tasks := queue.NewMemory(4)
	service := application.NewService(store, gacha.Registry{domain.GameGenshin: fakeSource{}}, tasks, ids, clock, logger)
	service.StartWorkers(ctx, 1)
	handler := New(service, store, logger, ids).Handler()

	body := bytes.NewBufferString(`{"game":"genshin","source_url":"https://example.invalid/?authkey=super-secret"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/sync-jobs", body)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", response.Code, response.Body.String())
	}
	var created domain.SyncJob
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.ID == "" || response.Header().Get("Location") == "" {
		t.Fatalf("invalid create response: %#v", created)
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		job, err := store.GetSyncJob(ctx, created.ID)
		if err != nil {
			t.Fatal(err)
		}
		if job.Status == domain.SyncCompleted {
			if job.ImportedCount != 1 || job.AccountID == "" {
				t.Fatalf("unexpected completed job: %#v", job)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("job did not complete: %#v", job)
		}
		time.Sleep(10 * time.Millisecond)
	}

	request = httptest.NewRequest(http.MethodGet, "/api/v1/sync-jobs/"+created.ID, nil)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	var finished domain.SyncJob
	if err := json.Unmarshal(response.Body.Bytes(), &finished); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		path   string
		status int
	}{
		{"/api/v1/accounts/" + finished.AccountID + "/summary", 200},
		{"/api/v1/accounts/" + finished.AccountID + "/pulls?rarity=5", 200},
		{"/api/v1/accounts/" + finished.AccountID + "/pulls?rarity=8", 400},
		{"/api/v1/accounts/missing/summary", 404}, {"/api/v1/missing", 404},
	} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", tt.path, nil))
		if w.Code != tt.status {
			t.Fatalf("%s: got %d: %s", tt.path, w.Code, w.Body.String())
		}
		if tt.path == "/api/v1/accounts/"+finished.AccountID+"/summary" {
			var summary domain.AccountSummary
			if err := json.Unmarshal(w.Body.Bytes(), &summary); err != nil {
				t.Fatal(err)
			}
			if summary.Total != 1 || summary.FiveStarCount != 1 || summary.LastImport == nil {
				t.Fatalf("summary: %#v", summary)
			}
		}
	}
}

func TestCrossOriginWriteRejected(t *testing.T) {
	handler := New(nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), platform.Generator{}).Handler()
	r := httptest.NewRequest("POST", "/api/v1/sync-jobs", bytes.NewBufferString(`{}`))
	r.Header.Set("Sec-Fetch-Site", "cross-site")
	r.Header.Set("Origin", "https://untrusted.example")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("got %d", w.Code)
	}
}

func TestRejectTrailingJSON(t *testing.T) {
	for _, body := range []string{`{} {}`, `{} broken`} {
		w := httptest.NewRecorder()
		if err := decodeJSON(w, httptest.NewRequest("POST", "/", bytes.NewBufferString(body)), &map[string]any{}); err == nil {
			t.Fatalf("accepted %q", body)
		}
	}
}

func TestInvalidJSONIsRejected(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, "file:"+filepath.Join(t.TempDir(), "api.db"), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ids := platform.Generator{}
	service := application.NewService(store, gacha.Registry{}, queue.NewMemory(1), ids, platform.Clock{}, logger)
	handler := New(service, store, logger, ids).Handler()

	request := httptest.NewRequest(http.MethodPost, "/api/v1/sync-jobs", bytes.NewBufferString(`{"game":`))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", response.Code)
	}
}
