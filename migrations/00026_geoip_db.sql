-- +goose Up
CREATE TABLE geoip_db (
    id smallint PRIMARY KEY CHECK (id = 1),
    month date NOT NULL CHECK (extract(day FROM month) = 1),
    data bytea NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE geoip_db;
