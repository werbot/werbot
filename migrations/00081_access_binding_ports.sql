-- +goose Up
-- Allowed target ports; empty means any port.
ALTER TABLE access_binding ADD COLUMN ports int4range[] NOT NULL DEFAULT '{}';

-- +goose Down
ALTER TABLE access_binding DROP COLUMN ports;
