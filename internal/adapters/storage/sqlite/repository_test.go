package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"gachaslop/internal/domain"
)

func TestStoreImportsIdempotentlyAndPaginates(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, "file:"+filepath.Join(t.TempDir(), "test.db"), 2)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	now := time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)
	batch := domain.ImportBatch{
		Account: domain.Account{ID: "account-1", Game: domain.GameGenshin, UID: "700000001", Region: "os_euro", Language: "ru-ru", CreatedAt: now, UpdatedAt: now},
		Pulls: []domain.Pull{
			{ID: "pull-3", ExternalID: "3", GachaType: "301", SourcePool: "301", PoolFamily: domain.PoolLimitedCharacter, PityGroup: "genshin:301", Name: "C", ItemType: "Weapon", Rarity: 3, PulledAt: "2026-09-14 12:00:03", CreatedAt: now},
			{ID: "pull-2", ExternalID: "2", GachaType: "301", SourcePool: "301", PoolFamily: domain.PoolLimitedCharacter, PityGroup: "genshin:301", Name: "B", ItemType: "Weapon", Rarity: 4, PulledAt: "2026-09-14 12:00:02", CreatedAt: now},
			{ID: "pull-1", ExternalID: "1", GachaType: "301", SourcePool: "301", PoolFamily: domain.PoolLimitedCharacter, PityGroup: "genshin:301", Name: "A", ItemType: "Character", Rarity: 5, PulledAt: "2026-09-14 12:00:01", CreatedAt: now},
		},
	}
	account, inserted, err := store.SaveImport(ctx, batch)
	if err != nil {
		t.Fatal(err)
	}
	if account.ID != "account-1" || inserted != 3 {
		t.Fatalf("unexpected import result: account=%#v inserted=%d", account, inserted)
	}
	_, inserted, err = store.SaveImport(ctx, batch)
	if err != nil {
		t.Fatal(err)
	}
	if inserted != 0 {
		t.Fatalf("duplicate import inserted %d rows", inserted)
	}

	first, err := store.ListPulls(ctx, account.ID, domain.PullFilter{Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Items) != 2 || first.Items[0].ExternalID != "3" || first.NextCursor == "" {
		t.Fatalf("unexpected first page: %#v", first)
	}
	second, err := store.ListPulls(ctx, account.ID, domain.PullFilter{Limit: 2, Cursor: first.NextCursor})
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Items) != 1 || second.Items[0].ExternalID != "1" || second.NextCursor != "" {
		t.Fatalf("unexpected second page: %#v", second)
	}

	latest, err := store.LatestExternalIDs(ctx, account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if latest["301"] != "3" {
		t.Fatalf("unexpected latest IDs: %#v", latest)
	}
	filtered, err := store.ListPulls(ctx, account.ID, domain.PullFilter{PoolFamily: domain.PoolLimitedCharacter, Rarity: 5, Limit: 1})
	if err != nil || len(filtered.Items) != 1 || filtered.Items[0].Rarity != 5 || filtered.NextCursor != "" {
		t.Fatalf("rarity filter: %#v, %v", filtered, err)
	}
	summary, err := store.AccountSummary(ctx, account.ID)
	if err != nil || summary.Total != 3 || summary.FiveStarCount != 1 || summary.FourStarCount != 1 || summary.LastImport != nil {
		t.Fatalf("summary: %#v, %v", summary, err)
	}
	if err := store.CreateSyncJob(ctx, domain.SyncJob{ID: "job", Game: domain.GameGenshin, Status: domain.SyncQueued, CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkSyncJobCompleted(ctx, "job", account.ID, 3, 3, now); err != nil {
		t.Fatal(err)
	}
	summary, err = store.AccountSummary(ctx, account.ID)
	if err != nil || summary.LastImport == nil || summary.LastImport.ImportedCount != 3 {
		t.Fatalf("last import: %#v, %v", summary, err)
	}
}

func TestSyncJobLifecycleAndStartupRecovery(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, "file:"+filepath.Join(t.TempDir(), "test.db"), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	now := time.Now().UTC()
	job := domain.SyncJob{ID: "job-1", Game: domain.GameWuWa, Status: domain.SyncQueued, CreatedAt: now}
	if err := store.CreateSyncJob(ctx, job); err != nil {
		t.Fatal(err)
	}
	if err := store.FailInterruptedSyncJobs(ctx, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	stored, err := store.GetSyncJob(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != domain.SyncFailed || stored.ErrorCode != "worker_interrupted" {
		t.Fatalf("unexpected recovered job: %#v", stored)
	}
}
