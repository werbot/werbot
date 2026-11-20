-- +goose Up
-- Org-defined command patterns that add classes to the built-in classification.
CREATE TABLE command_class_rule (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    org_id uuid NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    pattern text NOT NULL CHECK (length(pattern) BETWEEN 1 AND 255),
    classes text[] NOT NULL CHECK (cardinality(classes) > 0 AND classes <@ ARRAY['read','write','destructive','privilege','secrets','egress','opaque']::text[]),
    created_by uuid REFERENCES users ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (org_id, pattern)
);
CREATE TRIGGER command_class_rule_org_changed AFTER INSERT OR UPDATE OR DELETE ON command_class_rule FOR EACH ROW EXECUTE FUNCTION werbot_notify_org_changed();

-- +goose Down
DROP TABLE command_class_rule;
