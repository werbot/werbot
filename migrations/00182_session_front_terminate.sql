-- +goose Up
-- front is the entry of the session: ssh (and every stage 5-12 row) or web.
-- A terminate request stays until the gateway ends the session; the gateway
-- instance gets it by NOTIFY and again when it reconnects.
ALTER TABLE session
    ADD COLUMN front text NOT NULL DEFAULT 'ssh',
    ADD COLUMN terminate_requested_at timestamptz,
    ADD COLUMN terminate_reason text;
ALTER TABLE session ADD CONSTRAINT session_front_check CHECK (front IN ('ssh', 'web')) NOT VALID;
ALTER TABLE session VALIDATE CONSTRAINT session_front_check;
CREATE INDEX session_terminate_pending_idx ON session (gateway_instance_id)
    WHERE terminate_requested_at IS NOT NULL AND ended_at IS NULL;

-- +goose Down
DROP INDEX session_terminate_pending_idx;
ALTER TABLE session DROP COLUMN terminate_reason, DROP COLUMN terminate_requested_at, DROP COLUMN front;
