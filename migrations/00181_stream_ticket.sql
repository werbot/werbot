-- +goose Up
-- The composite key lets stream_ticket reference an instance of the same org only.
ALTER TABLE gateway_instance ADD CONSTRAINT gateway_instance_id_org_key UNIQUE (id, org_id);

-- A single-use ticket for one browser stream through one gateway instance:
-- observe watches a live session, web opens a terminal on a resource. It lives
-- 30s; target_server_id is the replica holding the browser's WebSocket.
-- Every reference stays inside the ticket's organization.
CREATE TABLE stream_ticket (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    org_id uuid NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    mode text NOT NULL CHECK (mode IN ('observe', 'web')),
    session_id uuid,
    resource_id uuid,
    user_id uuid NOT NULL,
    cols int NOT NULL DEFAULT 0,
    rows int NOT NULL DEFAULT 0,
    gateway_instance_id uuid NOT NULL,
    target_server_id uuid NOT NULL,
    client_ip inet,
    expires_at timestamptz NOT NULL,
    used_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK ((mode = 'observe') = (session_id IS NOT NULL) AND (mode = 'web') = (resource_id IS NOT NULL)),
    FOREIGN KEY (session_id, org_id) REFERENCES session(id, org_id) ON DELETE CASCADE,
    FOREIGN KEY (gateway_instance_id, org_id) REFERENCES gateway_instance(id, org_id) ON DELETE CASCADE,
    FOREIGN KEY (resource_id, org_id) REFERENCES resource(id, org_id) ON DELETE CASCADE,
    FOREIGN KEY (org_id, user_id) REFERENCES membership(org_id, user_id) ON DELETE CASCADE
);
CREATE INDEX stream_ticket_expires_idx ON stream_ticket (expires_at);

-- +goose Down
DROP TABLE stream_ticket;
ALTER TABLE gateway_instance DROP CONSTRAINT gateway_instance_id_org_key;
