package ports

import (
	"context"
	"time"

	"gachaslop/internal/domain"
)

type Repository interface {
	Ping(context.Context) error
	Close() error

	CreateSyncJob(context.Context, domain.SyncJob) error
	GetSyncJob(context.Context, string) (domain.SyncJob, error)
	MarkSyncJobRunning(context.Context, string, time.Time) error
	MarkSyncJobCompleted(context.Context, string, string, int, int, time.Time) error
	MarkSyncJobFailed(context.Context, string, string, string, time.Time) error
	FailInterruptedSyncJobs(context.Context, time.Time) error

	GetAccount(context.Context, string) (domain.Account, error)
	ListAccounts(context.Context) ([]domain.Account, error)
	LatestExternalIDs(context.Context, string) (map[string]string, error)
	SaveImport(context.Context, domain.ImportBatch) (domain.Account, int, error)
	ListPulls(context.Context, string, domain.PullFilter) (domain.PullPage, error)
	AllPulls(context.Context, string) ([]domain.Pull, error)
	AccountSummary(context.Context, string) (domain.AccountSummary, error)
}

type PullSource interface {
	Fetch(context.Context, string, map[string]string) (domain.ImportBatch, error)
}

type SourceRegistry interface {
	Source(domain.Game) (PullSource, bool)
}

type SyncTask struct {
	JobID     string
	Game      domain.Game
	SourceURL string
	AccountID string
}

type SyncQueue interface {
	Enqueue(SyncTask) error
	Start(context.Context, int, func(context.Context, SyncTask))
}

type IDGenerator interface {
	New() string
}

type Clock interface {
	Now() time.Time
}

// Banner ports are independent of private accounts and import credentials.
type BannerSource interface {
	FetchBanners(context.Context, domain.Game) (domain.BannerSchedule, error)
}

// Artwork is optional enrichment, independent of calendar availability.
type BannerArtwork interface {
	Enrich(context.Context, *domain.BannerSchedule)
}

type BannerRepository interface {
	GetBannerSchedule(context.Context, domain.Game) (domain.BannerSchedule, error)
	SaveBannerSchedule(context.Context, domain.BannerSchedule) error
}
