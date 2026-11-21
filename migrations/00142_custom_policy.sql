-- +goose Up
-- Hand-written Cedar policies of an organization.
CREATE TABLE custom_policy (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    org_id uuid NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    name text NOT NULL CHECK (length(name) BETWEEN 1 AND 63),
    cedar text NOT NULL CHECK (octet_length(cedar) BETWEEN 1 AND 65536),
    enabled bool NOT NULL DEFAULT true,
    created_by uuid REFERENCES users ON DELETE SET NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (org_id, name)
);
CREATE TRIGGER custom_policy_org_changed AFTER INSERT OR UPDATE OR DELETE ON custom_policy FOR EACH ROW EXECUTE FUNCTION werbot_notify_org_changed();

-- +goose Down
DROP TABLE custom_policy;
