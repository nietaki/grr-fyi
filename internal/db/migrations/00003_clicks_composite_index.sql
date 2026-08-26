-- +goose Up
CREATE INDEX IF NOT EXISTS idx_clicks_link_id_ip_hash ON clicks(link_id, ip_hash);

-- +goose Down
DROP INDEX IF EXISTS idx_clicks_link_id_ip_hash;
