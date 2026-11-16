-- +goose Up
CREATE TABLE team_member (
    team_id uuid NOT NULL,
    org_id uuid NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    user_id uuid NOT NULL,
    PRIMARY KEY (team_id, user_id),
    FOREIGN KEY (team_id, org_id) REFERENCES team (id, org_id) ON DELETE CASCADE,
    FOREIGN KEY (org_id, user_id) REFERENCES membership (org_id, user_id) ON DELETE CASCADE
);
CREATE INDEX team_member_user_idx ON team_member (org_id, user_id);

-- +goose Down
DROP TABLE team_member;
