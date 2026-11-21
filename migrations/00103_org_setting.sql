-- +goose Up
CREATE TABLE org_setting (
    org_id uuid PRIMARY KEY REFERENCES organization(id) ON DELETE CASCADE,
    cli_session_ttl interval NOT NULL DEFAULT '7 days' CHECK (cli_session_ttl BETWEEN '1 hour' AND '30 days'),
    cli_session_max interval NOT NULL DEFAULT '30 days' CHECK (cli_session_max BETWEEN '1 day' AND '90 days'),
    updated_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO org_setting (org_id) SELECT id FROM organization;

-- +goose Down
DROP TABLE org_setting;
