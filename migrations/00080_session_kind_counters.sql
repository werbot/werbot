-- +goose Up
-- kind defaults to 'terminal' so stage 5/6 rows and callers keep working.
ALTER TABLE session
    ADD COLUMN kind text NOT NULL DEFAULT 'terminal'
        CONSTRAINT session_kind_check CHECK (kind IN ('terminal', 'exec', 'stream', 'datagram')),
    ADD COLUMN target text NOT NULL DEFAULT '',
    ADD COLUMN bytes_up bigint NOT NULL DEFAULT 0,
    ADD COLUMN bytes_down bigint NOT NULL DEFAULT 0,
    ADD COLUMN datagrams_up bigint NOT NULL DEFAULT 0,
    ADD COLUMN datagrams_down bigint NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE session
    DROP COLUMN kind, DROP COLUMN target, DROP COLUMN bytes_up, DROP COLUMN bytes_down,
    DROP COLUMN datagrams_up, DROP COLUMN datagrams_down;
