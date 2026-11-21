-- +goose Up
-- Path rules: which paths a command may touch, by subject and resource.
CREATE TABLE path_rule (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    org_id uuid NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    name text NOT NULL CHECK (length(name) BETWEEN 1 AND 63),
    subjects jsonb NOT NULL CHECK (jsonb_typeof(subjects) = 'object'),
    resource_selector jsonb NOT NULL CHECK (jsonb_typeof(resource_selector) = 'object'),
    paths text[] NOT NULL CHECK (cardinality(paths) BETWEEN 1 AND 32 AND array_position(paths, NULL) IS NULL),
    access text NOT NULL CHECK (access IN ('read', 'write', 'any')),
    effect text NOT NULL CHECK (effect IN ('only', 'deny', 'approval')),
    message text NOT NULL DEFAULT '' CHECK (length(message) <= 256),
    created_by uuid REFERENCES users ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (org_id, name)
);
CREATE TRIGGER path_rule_org_changed AFTER INSERT OR UPDATE OR DELETE ON path_rule FOR EACH ROW EXECUTE FUNCTION werbot_notify_org_changed();

-- +goose Down
DROP TABLE path_rule;
