-- +goose Up
-- gateway.ws_url is the WebSocket endpoint of the pool ("" until set).
-- session.transport is the transport of the client connection: tcp (SSH
-- port), ws (WebSocket input) or internal (the web terminal); every older
-- web session is internal.
ALTER TABLE gateway ADD COLUMN ws_url text NOT NULL DEFAULT '';
ALTER TABLE session ADD COLUMN transport text NOT NULL DEFAULT 'tcp';
UPDATE session SET transport = 'internal' WHERE front = 'web';
ALTER TABLE session ADD CONSTRAINT session_transport_check CHECK (transport IN ('tcp', 'ws', 'internal')) NOT VALID;
ALTER TABLE session VALIDATE CONSTRAINT session_transport_check;

-- +goose Down
ALTER TABLE session DROP COLUMN transport;
ALTER TABLE gateway DROP COLUMN ws_url;
