CREATE INDEX idx_sync_jobs_account_completed
ON sync_jobs(account_id, completed_at DESC) WHERE status = 'completed';
