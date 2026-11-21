-- +goose Up
-- Paths a console or exec command touched; NULL when none were recorded.
ALTER TABLE session ADD COLUMN paths text[];

-- +goose Down
ALTER TABLE session DROP COLUMN paths;
