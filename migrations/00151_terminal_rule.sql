-- +goose Up
-- Terminal rules: whether a shell runs as a checked console or is denied.
CREATE TABLE terminal_rule (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    org_id uuid NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    name text NOT NULL CHECK (length(name) BETWEEN 1 AND 63),
    subjects jsonb NOT NULL CHECK (jsonb_typeof(subjects) = 'object'),
    resource_selector jsonb NOT NULL CHECK (jsonb_typeof(resource_selector) = 'object'),
    mode text NOT NULL CHECK (mode IN ('console', 'deny')),
    created_by uuid REFERENCES users ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (org_id, name)
);
CREATE TRIGGER terminal_rule_org_changed AFTER INSERT OR UPDATE OR DELETE ON terminal_rule FOR EACH ROW EXECUTE FUNCTION werbot_notify_org_changed();

-- +goose Down
DROP TABLE terminal_rule;
