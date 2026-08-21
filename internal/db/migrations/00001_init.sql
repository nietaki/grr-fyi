-- +goose Up
CREATE TABLE IF NOT EXISTS meta (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS meta;
