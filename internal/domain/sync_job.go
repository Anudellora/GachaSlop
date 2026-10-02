package domain

import "time"

type SyncStatus string

const (
	SyncQueued    SyncStatus = "queued"
	SyncRunning   SyncStatus = "running"
	SyncCompleted SyncStatus = "completed"
	SyncFailed    SyncStatus = "failed"
)

type SyncJob struct {
	ID            string     `json:"id"`
	Game          Game       `json:"game"`
	Status        SyncStatus `json:"status"`
	AccountID     string     `json:"account_id,omitempty"`
	FetchedCount  int        `json:"fetched_count"`
	ImportedCount int        `json:"imported_count"`
	ErrorCode     string     `json:"error_code,omitempty"`
	ErrorMessage  string     `json:"error_message,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
}
