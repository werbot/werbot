-- +goose Up
-- The verification state of the agent_readonly credential of a resource: none
-- (no such credential), pending (set or configuration changed, not yet
-- checked), ok, has_write or error (the latest check of the gateway), with a
-- short detail of the findings.
ALTER TABLE resource
    ADD COLUMN agent_readonly_status text NOT NULL DEFAULT 'none'
        CONSTRAINT resource_agent_readonly_status_check CHECK (agent_readonly_status IN ('none', 'pending', 'ok', 'has_write', 'error')),
    ADD COLUMN agent_readonly_detail text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE resource
    DROP COLUMN agent_readonly_detail,
    DROP COLUMN agent_readonly_status;
