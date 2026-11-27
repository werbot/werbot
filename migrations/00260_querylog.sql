-- +goose Up
-- recording.format is the artefact format: asciicast (terminal) or querylog
-- (NDJSON, one line per database operation). session.queries counts the
-- querylog lines of the session.
ALTER TABLE recording ADD COLUMN format text NOT NULL DEFAULT 'asciicast' CHECK (format IN ('asciicast', 'querylog'));
ALTER TABLE session ADD COLUMN queries bigint NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE session DROP COLUMN queries;
ALTER TABLE recording DROP COLUMN format;
