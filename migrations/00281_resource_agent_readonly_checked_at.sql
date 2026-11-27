-- +goose Up
-- When the gateway ended the check behind agent_readonly_status (clamped to
-- the server's clock); of two results of one check version the later wins.
-- NULL: no result stored yet.
ALTER TABLE resource ADD COLUMN agent_readonly_checked_at timestamptz;

-- +goose Down
ALTER TABLE resource DROP COLUMN agent_readonly_checked_at;
