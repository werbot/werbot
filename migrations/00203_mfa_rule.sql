-- +goose Up
-- MFA rules: which subjects need a fresh second factor for which resources.
CREATE TABLE mfa_rule (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    org_id uuid NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    name text NOT NULL CHECK (length(name) BETWEEN 1 AND 63),
    subjects jsonb NOT NULL CHECK (jsonb_typeof(subjects) = 'object'),
    resource_selector jsonb NOT NULL CHECK (jsonb_typeof(resource_selector) = 'object'),
    max_age interval NOT NULL CHECK (max_age BETWEEN '30 seconds' AND '24 hours'),
    strength text NOT NULL DEFAULT 'any' CHECK (strength IN ('any', 'webauthn')),
    created_by uuid REFERENCES users ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (org_id, name)
);
CREATE TRIGGER mfa_rule_org_changed AFTER INSERT OR UPDATE OR DELETE ON mfa_rule FOR EACH ROW EXECUTE FUNCTION werbot_notify_org_changed();

-- +goose Down
DROP TABLE mfa_rule;
