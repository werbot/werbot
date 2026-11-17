-- +goose Up
CREATE TABLE signing_key (
    id smallint PRIMARY KEY CHECK (id = 1),
    ciphertext bytea NOT NULL,
    key_id text NOT NULL,
    public_key bytea NOT NULL CHECK (length(public_key) = 32)
);

-- +goose Down
DROP TABLE signing_key;
