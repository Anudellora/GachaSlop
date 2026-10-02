package sqlite

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"gachaslop/internal/domain"
)

const accountColumns = `id, game, uid, region, language, timezone_offset, created_at, updated_at`
const pullColumns = `id, account_id, external_id, gacha_id, gacha_type, source_pool, pool_family, pity_group, item_id, name, item_type, rarity, pulled_at, created_at`
const jobColumns = `id, game, status, COALESCE(account_id, ''), fetched_count, imported_count, error_code, error_message, created_at, started_at, completed_at`

func (s *Store) CreateSyncJob(ctx context.Context, job domain.SyncJob) error {
	_, err := s.db.ExecContext(ctx, `
        INSERT INTO sync_jobs(id, game, status, account_id, created_at)
        VALUES (?, ?, ?, NULLIF(?, ''), ?)`,
		job.ID, job.Game, job.Status, job.AccountID, formatTime(job.CreatedAt),
	)
	if err != nil {
		return fmt.Errorf("create sync job: %w", err)
	}
	return nil
}

func (s *Store) GetSyncJob(ctx context.Context, id string) (domain.SyncJob, error) {
	row := s.db.QueryRowContext(ctx, "SELECT "+jobColumns+" FROM sync_jobs WHERE id = ?", id)
	job, err := scanJob(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.SyncJob{}, domain.ErrNotFound
	}
	return job, err
}

func (s *Store) MarkSyncJobRunning(ctx context.Context, id string, at time.Time) error {
	return s.updateJob(ctx, id, `status = ?, started_at = ?`, domain.SyncRunning, formatTime(at))
}

func (s *Store) MarkSyncJobCompleted(ctx context.Context, id, accountID string, fetched, imported int, at time.Time) error {
	return s.updateJob(ctx, id, `status = ?, account_id = ?, fetched_count = ?, imported_count = ?, completed_at = ?`,
		domain.SyncCompleted, accountID, fetched, imported, formatTime(at))
}

func (s *Store) MarkSyncJobFailed(ctx context.Context, id, code, message string, at time.Time) error {
	return s.updateJob(ctx, id, `status = ?, error_code = ?, error_message = ?, completed_at = ?`,
		domain.SyncFailed, code, message, formatTime(at))
}

func (s *Store) updateJob(ctx context.Context, id, set string, args ...any) error {
	args = append(args, id)
	result, err := s.db.ExecContext(ctx, "UPDATE sync_jobs SET "+set+" WHERE id = ?", args...)
	if err != nil {
		return fmt.Errorf("update sync job: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("sync job rows affected: %w", err)
	}
	if affected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (s *Store) FailInterruptedSyncJobs(ctx context.Context, at time.Time) error {
	_, err := s.db.ExecContext(ctx, `
        UPDATE sync_jobs
        SET status = 'failed', error_code = 'worker_interrupted',
            error_message = 'sync credential was held only in memory; start a new sync',
            completed_at = ?
        WHERE status IN ('queued', 'running')`, formatTime(at))
	if err != nil {
		return fmt.Errorf("fail interrupted sync jobs: %w", err)
	}
	return nil
}

func (s *Store) GetAccount(ctx context.Context, id string) (domain.Account, error) {
	row := s.db.QueryRowContext(ctx, "SELECT "+accountColumns+" FROM accounts WHERE id = ?", id)
	account, err := scanAccount(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Account{}, domain.ErrNotFound
	}
	return account, err
}

func (s *Store) ListAccounts(ctx context.Context) ([]domain.Account, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT "+accountColumns+" FROM accounts ORDER BY updated_at DESC, id")
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	defer rows.Close()

	accounts := make([]domain.Account, 0)
	for rows.Next() {
		account, err := scanAccount(rows)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, account)
	}
	return accounts, rows.Err()
}

func (s *Store) LatestExternalIDs(ctx context.Context, accountID string) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `
        SELECT source_pool, external_id
        FROM pulls
        WHERE account_id = ?
        ORDER BY pulled_at DESC, external_id DESC`, accountID)
	if err != nil {
		return nil, fmt.Errorf("latest pull ids: %w", err)
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var sourcePool, externalID string
		if err := rows.Scan(&sourcePool, &externalID); err != nil {
			return nil, fmt.Errorf("scan latest pull id: %w", err)
		}
		if _, exists := result[sourcePool]; !exists {
			result[sourcePool] = externalID
		}
	}
	return result, rows.Err()
}

func (s *Store) SaveImport(ctx context.Context, batch domain.ImportBatch) (domain.Account, int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Account{}, 0, fmt.Errorf("begin import: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	account, err := upsertAccount(ctx, tx, batch.Account)
	if err != nil {
		return domain.Account{}, 0, err
	}

	statement, err := tx.PrepareContext(ctx, `
        INSERT INTO pulls(
            id, account_id, external_id, gacha_id, gacha_type, source_pool, pool_family, pity_group,
            item_id, name, item_type, rarity, pulled_at, created_at
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(account_id, external_id) DO NOTHING`)
	if err != nil {
		return domain.Account{}, 0, fmt.Errorf("prepare pull insert: %w", err)
	}
	defer statement.Close()

	inserted := 0
	for _, pull := range batch.Pulls {
		result, err := statement.ExecContext(ctx,
			pull.ID, account.ID, pull.ExternalID, pull.GachaID, pull.GachaType, pull.SourcePool,
			pull.PoolFamily, pull.PityGroup, pull.ItemID, pull.Name, pull.ItemType, pull.Rarity,
			pull.PulledAt, formatTime(pull.CreatedAt),
		)
		if err != nil {
			return domain.Account{}, 0, fmt.Errorf("insert pull %s: %w", pull.ExternalID, err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return domain.Account{}, 0, fmt.Errorf("pull rows affected: %w", err)
		}
		inserted += int(affected)
	}

	if _, err := tx.ExecContext(ctx, "UPDATE accounts SET updated_at = ? WHERE id = ?", formatTime(batch.Account.UpdatedAt), account.ID); err != nil {
		return domain.Account{}, 0, fmt.Errorf("touch account: %w", err)
	}
	account.UpdatedAt = batch.Account.UpdatedAt
	if err := tx.Commit(); err != nil {
		return domain.Account{}, 0, fmt.Errorf("commit import: %w", err)
	}
	return account, inserted, nil
}

func upsertAccount(ctx context.Context, tx *sql.Tx, candidate domain.Account) (domain.Account, error) {
	row := tx.QueryRowContext(ctx, "SELECT "+accountColumns+" FROM accounts WHERE game = ? AND uid = ? AND region = ?",
		candidate.Game, candidate.UID, candidate.Region)
	account, err := scanAccount(row)
	if err == nil {
		_, err = tx.ExecContext(ctx, `
            UPDATE accounts SET language = ?, timezone_offset = ?, updated_at = ? WHERE id = ?`,
			candidate.Language, candidate.TimezoneOffset, formatTime(candidate.UpdatedAt), account.ID)
		if err != nil {
			return domain.Account{}, fmt.Errorf("update account: %w", err)
		}
		account.Language = candidate.Language
		account.TimezoneOffset = candidate.TimezoneOffset
		account.UpdatedAt = candidate.UpdatedAt
		return account, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return domain.Account{}, err
	}

	_, err = tx.ExecContext(ctx, `
        INSERT INTO accounts(id, game, uid, region, language, timezone_offset, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		candidate.ID, candidate.Game, candidate.UID, candidate.Region, candidate.Language,
		candidate.TimezoneOffset, formatTime(candidate.CreatedAt), formatTime(candidate.UpdatedAt),
	)
	if err != nil {
		return domain.Account{}, fmt.Errorf("insert account: %w", err)
	}
	return candidate, nil
}

func (s *Store) ListPulls(ctx context.Context, accountID string, filter domain.PullFilter) (domain.PullPage, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	where := []string{"account_id = ?"}
	args := []any{accountID}
	if filter.PoolFamily != "" {
		where = append(where, "pool_family = ?")
		args = append(args, filter.PoolFamily)
	}
	if filter.Rarity != 0 {
		where = append(where, "rarity = ?")
		args = append(args, filter.Rarity)
	}
	if filter.Cursor != "" {
		pulledAt, externalID, err := decodeCursor(filter.Cursor)
		if err != nil {
			return domain.PullPage{}, fmt.Errorf("%w: invalid cursor", domain.ErrInvalidInput)
		}
		where = append(where, "(pulled_at < ? OR (pulled_at = ? AND external_id < ?))")
		args = append(args, pulledAt, pulledAt, externalID)
	}
	args = append(args, limit+1)
	query := "SELECT " + pullColumns + " FROM pulls WHERE " + strings.Join(where, " AND ") +
		" ORDER BY pulled_at DESC, external_id DESC LIMIT ?"
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return domain.PullPage{}, fmt.Errorf("list pulls: %w", err)
	}
	defer rows.Close()

	items := make([]domain.Pull, 0, limit+1)
	for rows.Next() {
		pull, err := scanPull(rows)
		if err != nil {
			return domain.PullPage{}, err
		}
		items = append(items, pull)
	}
	if err := rows.Err(); err != nil {
		return domain.PullPage{}, err
	}
	page := domain.PullPage{Items: items}
	if len(items) > limit {
		last := items[limit-1]
		page.Items = items[:limit]
		page.NextCursor = encodeCursor(last.PulledAt, last.ExternalID)
	}
	return page, nil
}

func (s *Store) AccountSummary(ctx context.Context, accountID string) (domain.AccountSummary, error) {
	var result domain.AccountSummary
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*),
		COALESCE(SUM(CASE WHEN rarity = 5 THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN rarity = 4 THEN 1 ELSE 0 END), 0)
		FROM pulls WHERE account_id = ?`, accountID).
		Scan(&result.Total, &result.FiveStarCount, &result.FourStarCount)
	if err != nil {
		return result, fmt.Errorf("account summary: %w", err)
	}
	job, err := scanJob(s.db.QueryRowContext(ctx, "SELECT "+jobColumns+
		" FROM sync_jobs WHERE account_id = ? AND status = 'completed' ORDER BY completed_at DESC, id DESC LIMIT 1", accountID))
	if err == nil {
		result.LastImport = &job
	} else if !errors.Is(err, sql.ErrNoRows) {
		return result, fmt.Errorf("last import: %w", err)
	}
	return result, nil
}

func (s *Store) AllPulls(ctx context.Context, accountID string) ([]domain.Pull, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT "+pullColumns+" FROM pulls WHERE account_id = ? ORDER BY pulled_at DESC, external_id DESC", accountID)
	if err != nil {
		return nil, fmt.Errorf("all pulls: %w", err)
	}
	defer rows.Close()
	items := make([]domain.Pull, 0)
	for rows.Next() {
		pull, err := scanPull(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, pull)
	}
	return items, rows.Err()
}

type scanner interface {
	Scan(...any) error
}

func scanAccount(row scanner) (domain.Account, error) {
	var account domain.Account
	var createdAt, updatedAt string
	if err := row.Scan(&account.ID, &account.Game, &account.UID, &account.Region, &account.Language,
		&account.TimezoneOffset, &createdAt, &updatedAt); err != nil {
		return domain.Account{}, err
	}
	var err error
	if account.CreatedAt, err = parseTime(createdAt); err != nil {
		return domain.Account{}, err
	}
	if account.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return domain.Account{}, err
	}
	return account, nil
}

func scanPull(row scanner) (domain.Pull, error) {
	var pull domain.Pull
	var createdAt string
	if err := row.Scan(&pull.ID, &pull.AccountID, &pull.ExternalID, &pull.GachaID, &pull.GachaType,
		&pull.SourcePool, &pull.PoolFamily, &pull.PityGroup, &pull.ItemID, &pull.Name, &pull.ItemType, &pull.Rarity,
		&pull.PulledAt, &createdAt); err != nil {
		return domain.Pull{}, err
	}
	var err error
	pull.CreatedAt, err = parseTime(createdAt)
	return pull, err
}

func scanJob(row scanner) (domain.SyncJob, error) {
	var job domain.SyncJob
	var createdAt string
	var startedAt, completedAt sql.NullString
	if err := row.Scan(&job.ID, &job.Game, &job.Status, &job.AccountID, &job.FetchedCount,
		&job.ImportedCount, &job.ErrorCode, &job.ErrorMessage, &createdAt, &startedAt, &completedAt); err != nil {
		return domain.SyncJob{}, err
	}
	var err error
	if job.CreatedAt, err = parseTime(createdAt); err != nil {
		return domain.SyncJob{}, err
	}
	if startedAt.Valid {
		value, err := parseTime(startedAt.String)
		if err != nil {
			return domain.SyncJob{}, err
		}
		job.StartedAt = &value
	}
	if completedAt.Valid {
		value, err := parseTime(completedAt.String)
		if err != nil {
			return domain.SyncJob{}, err
		}
		job.CompletedAt = &value
	}
	return job, nil
}

func formatTime(value time.Time) string { return value.UTC().Format(time.RFC3339Nano) }

func parseTime(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse stored time: %w", err)
	}
	return parsed, nil
}

func encodeCursor(pulledAt, externalID string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(pulledAt + "\x00" + externalID))
}

func decodeCursor(cursor string) (string, string, error) {
	value, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return "", "", err
	}
	parts := strings.SplitN(string(value), "\x00", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", errors.New("invalid cursor")
	}
	return parts[0], parts[1], nil
}
