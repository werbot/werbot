-- +goose Up
-- Approval policies: who may request access to which resources, who approves, and how.
CREATE TABLE approval_policy (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    org_id uuid NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    name text NOT NULL CHECK (length(name) BETWEEN 1 AND 63),
    resource_selector jsonb NOT NULL,
    requesters jsonb NOT NULL,
    approvers jsonb NOT NULL DEFAULT '{}',
    mode text NOT NULL CHECK (mode IN ('manual', 'auto')),
    max_duration interval NOT NULL DEFAULT '8 hours' CHECK (max_duration BETWEEN '1 minute' AND '14 days'),
    require_reason bool NOT NULL DEFAULT true,
    four_eyes bool NOT NULL DEFAULT false,
    break_glass bool NOT NULL DEFAULT false,
    created_by uuid REFERENCES users ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (org_id, name)
);

-- +goose Down
DROP TABLE approval_policy;
