-- +goose Up
ALTER TABLE resource
    ADD COLUMN idle_timeout interval NULL CHECK (idle_timeout BETWEEN '1 minute' AND '24 hours'),
    ADD COLUMN max_duration interval NULL CHECK (max_duration BETWEEN '1 minute' AND '168 hours');

-- +goose Down
ALTER TABLE resource DROP COLUMN idle_timeout, DROP COLUMN max_duration;
