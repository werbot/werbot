-- +goose Up
-- The composite key lets recording reference a session of the same org only.
ALTER TABLE session ADD CONSTRAINT session_id_org_key UNIQUE (id, org_id);

CREATE TABLE recording (
    session_id uuid PRIMARY KEY,
    org_id uuid NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    store text NOT NULL CHECK (store IN ('fs', 's3')),
    chunks int NOT NULL DEFAULT 0 CHECK (chunks >= 0),
    bytes bigint NOT NULL DEFAULT 0 CHECK (bytes >= 0),
    -- hashchain.Genesis(), then the hash of the last accepted chunk
    last_hash bytea NOT NULL DEFAULT decode(repeat('00', 32), 'hex') CHECK (length(last_hash) = 32),
    final_hash bytea CHECK (length(final_hash) = 32),
    manifest_signature bytea CHECK (length(manifest_signature) = 64),
    verified_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (session_id, org_id) REFERENCES session(id, org_id) ON DELETE CASCADE
);
CREATE INDEX recording_org_id_idx ON recording (org_id);
-- no recording_chunk table: chunks are store objects, the chain is recording.last_hash

-- +goose Down
DROP TABLE recording;
ALTER TABLE session DROP CONSTRAINT session_id_org_key;
