-- +goose Up
-- Time-limited access to one resource, issued by an approval or break-glass.
CREATE TABLE access_grant (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    org_id uuid NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    approval_id uuid REFERENCES approval ON DELETE SET NULL,
    user_id uuid NOT NULL,
    resource_id uuid NOT NULL,
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    break_glass bool NOT NULL DEFAULT false,
    needs_review bool NOT NULL DEFAULT false,
    created_by uuid REFERENCES users ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (id, org_id),
    FOREIGN KEY (resource_id, org_id) REFERENCES resource(id, org_id) ON DELETE CASCADE,
    FOREIGN KEY (org_id, user_id) REFERENCES membership(org_id, user_id) ON DELETE CASCADE
);
CREATE INDEX access_grant_active_idx ON access_grant (org_id, expires_at) WHERE revoked_at IS NULL;
CREATE TRIGGER access_grant_org_changed AFTER INSERT OR UPDATE OR DELETE ON access_grant FOR EACH ROW EXECUTE FUNCTION werbot_notify_org_changed();

-- +goose Down
DROP TABLE access_grant;
