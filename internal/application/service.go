package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"gachaslop/internal/domain"
	"gachaslop/internal/ports"
)

type Service struct {
	repository ports.Repository
	sources    ports.SourceRegistry
	queue      ports.SyncQueue
	ids        ports.IDGenerator
	clock      ports.Clock
	logger     *slog.Logger
}

func NewService(
	repository ports.Repository,
	sources ports.SourceRegistry,
	queue ports.SyncQueue,
	ids ports.IDGenerator,
	clock ports.Clock,
	logger *slog.Logger,
) *Service {
	return &Service{
		repository: repository,
		sources:    sources,
		queue:      queue,
		ids:        ids,
		clock:      clock,
		logger:     logger,
	}
}

type CreateSyncCommand struct {
	Game      string
	SourceURL string
	AccountID string
}

func (s *Service) CreateSync(ctx context.Context, command CreateSyncCommand) (domain.SyncJob, error) {
	game, err := domain.ParseGame(command.Game)
	if err != nil {
		return domain.SyncJob{}, fmt.Errorf("%w: %v", domain.ErrInvalidInput, err)
	}
	command.SourceURL = strings.TrimSpace(command.SourceURL)
	if command.SourceURL == "" {
		return domain.SyncJob{}, fmt.Errorf("%w: source_url is required", domain.ErrInvalidInput)
	}
	if _, ok := s.sources.Source(game); !ok {
		return domain.SyncJob{}, fmt.Errorf("%w: no source adapter for %s", domain.ErrInvalidInput, game)
	}
	if command.AccountID != "" {
		account, err := s.repository.GetAccount(ctx, command.AccountID)
		if err != nil {
			return domain.SyncJob{}, err
		}
		if account.Game != game {
			return domain.SyncJob{}, fmt.Errorf("%w: account belongs to %s", domain.ErrAccountMismatch, account.Game)
		}
	}

	job := domain.SyncJob{
		ID:        s.ids.New(),
		Game:      game,
		Status:    domain.SyncQueued,
		AccountID: command.AccountID,
		CreatedAt: s.clock.Now(),
	}
	if err := s.repository.CreateSyncJob(ctx, job); err != nil {
		return domain.SyncJob{}, err
	}
	task := ports.SyncTask{JobID: job.ID, Game: game, SourceURL: command.SourceURL, AccountID: command.AccountID}
	if err := s.queue.Enqueue(task); err != nil {
		_ = s.repository.MarkSyncJobFailed(ctx, job.ID, "queue_full", "sync queue is full; retry later", s.clock.Now())
		return domain.SyncJob{}, err
	}
	return job, nil
}

func (s *Service) StartWorkers(ctx context.Context, workers int) {
	s.queue.Start(ctx, workers, s.processSync)
}

func (s *Service) processSync(ctx context.Context, task ports.SyncTask) {
	startedAt := s.clock.Now()
	if err := s.repository.MarkSyncJobRunning(ctx, task.JobID, startedAt); err != nil {
		s.logger.Error("mark sync running", "job_id", task.JobID, "error", err)
		return
	}

	var expected domain.Account
	stopIDs := map[string]string(nil)
	if task.AccountID != "" {
		var err error
		expected, err = s.repository.GetAccount(ctx, task.AccountID)
		if err != nil {
			s.failJob(task.JobID, err)
			return
		}
		stopIDs, err = s.repository.LatestExternalIDs(ctx, task.AccountID)
		if err != nil {
			s.failJob(task.JobID, err)
			return
		}
	}

	source, ok := s.sources.Source(task.Game)
	if !ok {
		s.failJob(task.JobID, fmt.Errorf("%w: source adapter disappeared", domain.ErrInvalidInput))
		return
	}
	batch, err := source.Fetch(ctx, task.SourceURL, stopIDs)
	if err != nil {
		s.failJob(task.JobID, err)
		return
	}
	if expected.ID != "" {
		if batch.Account.UID == "" {
			batch.Account.UID = expected.UID
		}
		if batch.Account.Region == "" {
			batch.Account.Region = expected.Region
		}
		if batch.Account.UID != expected.UID || batch.Account.Game != expected.Game ||
			(batch.Account.Region != "" && expected.Region != "" && batch.Account.Region != expected.Region) {
			s.failJob(task.JobID, domain.ErrAccountMismatch)
			return
		}
	}
	if batch.Account.UID == "" {
		s.failJob(task.JobID, fmt.Errorf("%w: could not determine account UID", domain.ErrInvalidInput))
		return
	}

	now := s.clock.Now()
	batch.Account.ID = s.ids.New()
	batch.Account.CreatedAt = now
	batch.Account.UpdatedAt = now
	for index := range batch.Pulls {
		batch.Pulls[index].ID = s.ids.New()
		batch.Pulls[index].CreatedAt = now
	}
	account, imported, err := s.repository.SaveImport(ctx, batch)
	if err != nil {
		s.failJob(task.JobID, err)
		return
	}
	if err := s.repository.MarkSyncJobCompleted(ctx, task.JobID, account.ID, len(batch.Pulls), imported, s.clock.Now()); err != nil {
		s.logger.Error("mark sync completed", "job_id", task.JobID, "error", err)
		return
	}
	s.logger.Info("sync completed", "job_id", task.JobID, "game", task.Game, "account_id", account.ID,
		"fetched", len(batch.Pulls), "imported", imported)
}

func (s *Service) failJob(jobID string, cause error) {
	code, message := classifyError(cause)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := s.repository.MarkSyncJobFailed(ctx, jobID, code, message, s.clock.Now()); err != nil {
		s.logger.Error("mark sync failed", "job_id", jobID, "error", err)
	}
	s.logger.Warn("sync failed", "job_id", jobID, "code", code, "error", cause)
}

func classifyError(err error) (string, string) {
	switch {
	case errors.Is(err, domain.ErrSourceExpired):
		return "source_expired", "history link is invalid or expired; open the history page again"
	case errors.Is(err, domain.ErrAccountMismatch):
		return "account_mismatch", "the history link belongs to another game account"
	case errors.Is(err, domain.ErrInvalidInput):
		return "invalid_source", safeError(err)
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return "worker_interrupted", "synchronization was interrupted; retry with a fresh history link"
	default:
		return "sync_failed", safeError(err)
	}
}

func safeError(err error) string {
	message := strings.TrimSpace(err.Error())
	if len(message) > 500 {
		message = message[:500]
	}
	return message
}

func (s *Service) GetSyncJob(ctx context.Context, id string) (domain.SyncJob, error) {
	return s.repository.GetSyncJob(ctx, id)
}

func (s *Service) GetAccount(ctx context.Context, id string) (domain.Account, error) {
	return s.repository.GetAccount(ctx, id)
}

func (s *Service) ListAccounts(ctx context.Context) ([]domain.Account, error) {
	return s.repository.ListAccounts(ctx)
}

func (s *Service) ListPulls(ctx context.Context, accountID string, filter domain.PullFilter) (domain.PullPage, error) {
	if _, err := s.repository.GetAccount(ctx, accountID); err != nil {
		return domain.PullPage{}, err
	}
	return s.repository.ListPulls(ctx, accountID, filter)
}

func (s *Service) Pity(ctx context.Context, accountID string) ([]domain.PoolStats, error) {
	if _, err := s.repository.GetAccount(ctx, accountID); err != nil {
		return nil, err
	}
	pulls, err := s.repository.AllPulls(ctx, accountID)
	if err != nil {
		return nil, err
	}
	return domain.CalculatePity(pulls), nil
}

func (s *Service) Summary(ctx context.Context, accountID string) (domain.AccountSummary, error) {
	if _, err := s.repository.GetAccount(ctx, accountID); err != nil {
		return domain.AccountSummary{}, err
	}
	return s.repository.AccountSummary(ctx, accountID)
}
