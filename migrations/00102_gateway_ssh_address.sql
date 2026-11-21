-- +goose Up
-- host:port clients dial for this pool; empty until an admin sets it.
ALTER TABLE gateway ADD COLUMN ssh_address text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE gateway DROP COLUMN ssh_address;
