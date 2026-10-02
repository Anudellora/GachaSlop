CREATE TABLE accounts (
    id TEXT PRIMARY KEY,
    game TEXT NOT NULL CHECK (game IN ('genshin', 'hsr', 'zzz', 'wuwa')),
    uid TEXT NOT NULL,
    region TEXT NOT NULL DEFAULT '',
    language TEXT NOT NULL DEFAULT '',
    timezone_offset INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE (game, uid, region)
);

CREATE TABLE pulls (
    id TEXT PRIMARY KEY,
    account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    external_id TEXT NOT NULL,
    gacha_id TEXT NOT NULL DEFAULT '',
    gacha_type TEXT NOT NULL,
    source_pool TEXT NOT NULL,
    pool_family TEXT NOT NULL,
    pity_group TEXT NOT NULL,
    item_id TEXT NOT NULL DEFAULT '',
    name TEXT NOT NULL,
    item_type TEXT NOT NULL,
    rarity INTEGER NOT NULL CHECK (rarity BETWEEN 1 AND 6),
    pulled_at TEXT NOT NULL,
    created_at TEXT NOT NULL,
    UNIQUE (account_id, external_id)
);

CREATE INDEX idx_pulls_account_time
    ON pulls (account_id, pulled_at DESC, external_id DESC);

CREATE INDEX idx_pulls_account_family_time
    ON pulls (account_id, pool_family, pulled_at DESC, external_id DESC);

CREATE INDEX idx_pulls_account_source_pool_time
    ON pulls (account_id, source_pool, pulled_at DESC, external_id DESC);

CREATE INDEX idx_pulls_account_pity_group_time
    ON pulls (account_id, pity_group, pulled_at DESC, external_id DESC);

CREATE TABLE sync_jobs (
    id TEXT PRIMARY KEY,
    game TEXT NOT NULL CHECK (game IN ('genshin', 'hsr', 'zzz', 'wuwa')),
    status TEXT NOT NULL CHECK (status IN ('queued', 'running', 'completed', 'failed')),
    account_id TEXT REFERENCES accounts(id) ON DELETE SET NULL,
    fetched_count INTEGER NOT NULL DEFAULT 0,
    imported_count INTEGER NOT NULL DEFAULT 0,
    error_code TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    started_at TEXT,
    completed_at TEXT
);

CREATE INDEX idx_sync_jobs_created_at ON sync_jobs (created_at DESC);
