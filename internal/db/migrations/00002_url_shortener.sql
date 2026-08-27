-- +goose Up
CREATE TABLE IF NOT EXISTS links (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    slug TEXT NOT NULL UNIQUE,
    target_url TEXT NOT NULL,
    claim_key_hash TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    revoked_at TEXT
);

CREATE TABLE IF NOT EXISTS clicks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    link_id INTEGER NOT NULL REFERENCES links(id) ON DELETE CASCADE,
    clicked_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    ip_hash TEXT,
    referrer TEXT,
    country TEXT
);

CREATE INDEX IF NOT EXISTS idx_clicks_link_id ON clicks(link_id);

CREATE TABLE IF NOT EXISTS slug_sequence (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    next_value INTEGER NOT NULL DEFAULT 1
);

INSERT INTO slug_sequence (id, next_value) VALUES (1, 4096);

-- +goose Down
DROP INDEX IF EXISTS idx_clicks_link_id;
DROP TABLE IF EXISTS clicks;
DROP TABLE IF EXISTS links;
DROP TABLE IF EXISTS slug_sequence;
