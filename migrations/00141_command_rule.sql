-- +goose Up
-- Command rules: who may run which command classes on which resources.
CREATE TABLE command_rule (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    org_id uuid NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    name text NOT NULL CHECK (length(name) BETWEEN 1 AND 63),
    subjects jsonb NOT NULL CHECK (jsonb_typeof(subjects) = 'object'),
    resource_selector jsonb NOT NULL CHECK (jsonb_typeof(resource_selector) = 'object'),
    classes text[] NOT NULL CHECK (cardinality(classes) > 0 AND classes <@ ARRAY['read','write','destructive','privilege','secrets','egress','opaque']::text[]),
    effect text NOT NULL CHECK (effect IN ('allow', 'deny', 'approval')),
    message text NOT NULL DEFAULT '' CHECK (length(message) <= 256),
    created_by uuid REFERENCES users ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (org_id, name)
);
CREATE TRIGGER command_rule_org_changed AFTER INSERT OR UPDATE OR DELETE ON command_rule FOR EACH ROW EXECUTE FUNCTION werbot_notify_org_changed();

-- +goose Down
DROP TABLE command_rule;
