-- +goose Up
CREATE TABLE ip_blocklist (
    cidr cidr PRIMARY KEY,
    source text NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ip_blocklist_gist ON ip_blocklist USING gist (cidr inet_ops);

-- +goose Down
DROP TABLE ip_blocklist;
