-- +goose Up
CREATE TABLE ca_key (
    id smallint PRIMARY KEY CHECK (id = 1),
    ciphertext bytea NOT NULL,
    key_id text NOT NULL,
    cert_pem text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE ca_key;
