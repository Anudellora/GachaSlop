package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gachaslop/internal/application"
	"gachaslop/internal/domain"
	"gachaslop/internal/ports"
)

type API struct {
	service    *application.Service
	repository ports.Repository
	logger     *slog.Logger
	ids        ports.IDGenerator
	banners    *application.BannerService
}

func (a *API) WithBanners(service *application.BannerService) *API { a.banners = service; return a }

func New(service *application.Service, repository ports.Repository, logger *slog.Logger, ids ports.IDGenerator) *API {
	return &API{service: service, repository: repository, logger: logger, ids: ids}
}

func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", a.info)
	mux.HandleFunc("GET /healthz", a.health)
	mux.HandleFunc("GET /readyz", a.ready)
	mux.HandleFunc("POST /api/v1/sync-jobs", a.createSyncJob)
	mux.HandleFunc("GET /api/v1/sync-jobs/{id}", a.getSyncJob)
	mux.HandleFunc("GET /api/v1/accounts", a.listAccounts)
	mux.HandleFunc("GET /api/v1/accounts/{id}", a.getAccount)
	mux.HandleFunc("GET /api/v1/accounts/{id}/pulls", a.listPulls)
	mux.HandleFunc("GET /api/v1/accounts/{id}/pity", a.getPity)
	mux.HandleFunc("GET /api/v1/accounts/{id}/summary", a.getSummary)
	mux.HandleFunc("GET /api/v1/banners", a.listBanners)

	return a.requestID(a.accessLog(a.recoverPanic(a.securityHeaders(http.NewCrossOriginProtection().Handler(mux)))))
}

func (a *API) listBanners(w http.ResponseWriter, r *http.Request) {
	game, err := domain.ParseGame(r.URL.Query().Get("game"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_game", "game must be genshin, hsr, zzz or wuwa")
		return
	}
	if a.banners == nil {
		writeError(w, http.StatusServiceUnavailable, "banners_unavailable", "banner service unavailable")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	schedule, err := a.banners.List(ctx, game)
	if err != nil {
		w.Header().Set("Retry-After", "60")
		writeError(w, http.StatusServiceUnavailable, "banners_unavailable", "could not load banner schedule; retry later")
		return
	}
	writeJSON(w, http.StatusOK, schedule)
}

func (a *API) info(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"name":    "GachaSLop API",
		"version": "0.1.0",
	})
}

func (a *API) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()
	if err := a.repository.Ping(ctx); err != nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "database is unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (a *API) createSyncJob(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Game      string `json:"game"`
		SourceURL string `json:"source_url"`
		AccountID string `json:"account_id"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	job, err := a.service.CreateSync(r.Context(), application.CreateSyncCommand{
		Game: input.Game, SourceURL: input.SourceURL, AccountID: input.AccountID,
	})
	if err != nil {
		a.writeApplicationError(w, err)
		return
	}
	w.Header().Set("Location", "/api/v1/sync-jobs/"+job.ID)
	writeJSON(w, http.StatusAccepted, job)
}

func (a *API) getSyncJob(w http.ResponseWriter, r *http.Request) {
	job, err := a.service.GetSyncJob(r.Context(), r.PathValue("id"))
	if err != nil {
		a.writeApplicationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (a *API) listAccounts(w http.ResponseWriter, r *http.Request) {
	accounts, err := a.service.ListAccounts(r.Context())
	if err != nil {
		a.writeApplicationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": accounts})
}

func (a *API) getAccount(w http.ResponseWriter, r *http.Request) {
	account, err := a.service.GetAccount(r.Context(), r.PathValue("id"))
	if err != nil {
		a.writeApplicationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, account)
}

func (a *API) listPulls(w http.ResponseWriter, r *http.Request) {
	filter := domain.PullFilter{Cursor: r.URL.Query().Get("cursor")}
	if raw := r.URL.Query().Get("rarity"); raw != "" {
		rarity, err := strconv.Atoi(raw)
		if err != nil || rarity < 3 || rarity > 5 {
			writeError(w, http.StatusBadRequest, "invalid_rarity", "rarity must be 3, 4 or 5")
			return
		}
		filter.Rarity = rarity
	}
	if raw := r.URL.Query().Get("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > 200 {
			writeError(w, http.StatusBadRequest, "invalid_limit", "limit must be between 1 and 200")
			return
		}
		filter.Limit = limit
	}
	if raw := r.URL.Query().Get("pool_family"); raw != "" {
		filter.PoolFamily = domain.PoolFamily(raw)
		if !validPoolFamily(filter.PoolFamily) {
			writeError(w, http.StatusBadRequest, "invalid_pool_family", "unknown pool_family")
			return
		}
	}
	page, err := a.service.ListPulls(r.Context(), r.PathValue("id"), filter)
	if err != nil {
		a.writeApplicationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (a *API) getPity(w http.ResponseWriter, r *http.Request) {
	stats, err := a.service.Pity(r.Context(), r.PathValue("id"))
	if err != nil {
		a.writeApplicationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": stats})
}

func (a *API) getSummary(w http.ResponseWriter, r *http.Request) {
	summary, err := a.service.Summary(r.Context(), r.PathValue("id"))
	if err != nil {
		a.writeApplicationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (a *API) writeApplicationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "resource was not found")
	case errors.Is(err, domain.ErrQueueFull):
		writeError(w, http.StatusServiceUnavailable, "queue_full", "sync queue is full; retry later")
	case errors.Is(err, domain.ErrAccountMismatch):
		writeError(w, http.StatusConflict, "account_mismatch", "account does not match the selected game")
	case errors.Is(err, domain.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, "invalid_input", publicError(err))
	default:
		a.logger.Error("request failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

func validPoolFamily(value domain.PoolFamily) bool {
	switch value {
	case domain.PoolBeginner, domain.PoolStandard, domain.PoolLimitedCharacter,
		domain.PoolLimitedWeapon, domain.PoolChronicled, domain.PoolBangboo, domain.PoolOther:
		return true
	default:
		return false
	}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("invalid request body")
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("request body must contain one JSON object")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

func publicError(err error) string {
	message := strings.TrimSpace(err.Error())
	message = strings.TrimPrefix(message, domain.ErrInvalidInput.Error()+": ")
	if len(message) > 300 {
		message = message[:300]
	}
	return message
}

type contextKey string

const requestIDKey contextKey = "request_id"

func (a *API) requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := a.ids.New()
		w.Header().Set("X-Request-ID", requestID)
		ctx := context.WithValue(r.Context(), requestIDKey, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *API) accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		a.logger.Info("http request",
			"request_id", r.Context().Value(requestIDKey),
			"method", r.Method,
			"path", r.URL.Path,
			"status", recorder.status,
			"duration_ms", time.Since(started).Milliseconds(),
		)
	})
}

func (a *API) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				a.logger.Error("panic recovered", "request_id", r.Context().Value(requestIDKey), "panic", recovered)
				writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (a *API) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *statusRecorder) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
