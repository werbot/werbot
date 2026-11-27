-- +goose Up
-- credential.slot names the credential a resource holds for one purpose:
-- primary (the session credential) or agent_readonly (stage 18). A resource has
-- one credential per slot; client_cert is a PEM certificate and key.
ALTER TABLE credential
    ADD COLUMN slot text NOT NULL DEFAULT 'primary' CHECK (slot IN ('primary', 'agent_readonly')),
    DROP CONSTRAINT credential_resource_id_key,
    ADD CONSTRAINT credential_resource_id_slot_key UNIQUE (resource_id, slot),
    DROP CONSTRAINT credential_kind_check,
    ADD CONSTRAINT credential_kind_check CHECK (kind IN ('password', 'private_key', 'client_cert'));

-- +goose Down
DELETE FROM credential WHERE slot <> 'primary' OR kind = 'client_cert';
ALTER TABLE credential
    DROP CONSTRAINT credential_kind_check,
    ADD CONSTRAINT credential_kind_check CHECK (kind IN ('password', 'private_key')),
    DROP CONSTRAINT credential_resource_id_slot_key,
    ADD CONSTRAINT credential_resource_id_key UNIQUE (resource_id),
    DROP COLUMN slot;
