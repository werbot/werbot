-- +goose Up
-- Access and command approvals. History survives a deleted resource or policy.
CREATE TABLE approval (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    org_id uuid NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    kind text NOT NULL CHECK (kind IN ('access', 'command')),
    requester_user_id uuid NOT NULL,
    agent_run_id uuid,
    resource_id uuid,
    label_selector jsonb,
    duration interval,
    command text,
    command_sha256 bytea,
    classes text[],
    reason text NOT NULL DEFAULT '',
    policy_id uuid REFERENCES approval_policy ON DELETE SET NULL,
    break_glass bool NOT NULL DEFAULT false,
    status text NOT NULL CHECK (status IN ('pending', 'approved', 'denied', 'expired', 'canceled')),
    decided_by uuid REFERENCES users ON DELETE SET NULL,
    decided_at timestamptz,
    comment text NOT NULL DEFAULT '',
    expires_at timestamptz NOT NULL,
    gateway_instance_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (id, org_id),
    FOREIGN KEY (org_id, requester_user_id) REFERENCES membership(org_id, user_id) ON DELETE CASCADE,
    FOREIGN KEY (resource_id, org_id) REFERENCES resource(id, org_id) ON DELETE SET NULL (resource_id),
    -- "exactly one" is checked by the service: deleting a resource nulls resource_id
    CHECK (num_nonnulls(resource_id, label_selector) <= 1)
);
CREATE INDEX approval_org_status_idx ON approval (org_id, status, created_at DESC);
CREATE INDEX approval_command_pending_idx ON approval (gateway_instance_id) WHERE kind = 'command' AND status = 'pending';

-- +goose Down
DROP TABLE approval;
