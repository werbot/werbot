-- +goose Up
ALTER TABLE org_setting
    ADD COLUMN grant_default_ttl interval NOT NULL DEFAULT '8 hours',
    ADD COLUMN grant_max_ttl interval NOT NULL DEFAULT '14 days' CHECK (grant_max_ttl BETWEEN '1 minute' AND '14 days'),
    ADD COLUMN request_pending_ttl interval NOT NULL DEFAULT '1 hour' CHECK (request_pending_ttl BETWEEN '1 minute' AND '24 hours'),
    ADD COLUMN command_approval_ttl interval NOT NULL DEFAULT '5 minutes' CHECK (command_approval_ttl BETWEEN '30 seconds' AND '1 hour'),
    ADD COLUMN lock_mode text NOT NULL DEFAULT 'best_effort' CHECK (lock_mode IN ('best_effort', 'strict')),
    ADD CONSTRAINT org_setting_grant_ttl CHECK (grant_default_ttl > '0' AND grant_default_ttl <= grant_max_ttl);
CREATE TRIGGER org_setting_org_changed AFTER INSERT OR UPDATE OR DELETE ON org_setting FOR EACH ROW EXECUTE FUNCTION werbot_notify_org_changed();

-- +goose Down
DROP TRIGGER org_setting_org_changed ON org_setting;
ALTER TABLE org_setting
    DROP CONSTRAINT org_setting_grant_ttl,
    DROP COLUMN lock_mode,
    DROP COLUMN command_approval_ttl,
    DROP COLUMN request_pending_ttl,
    DROP COLUMN grant_max_ttl,
    DROP COLUMN grant_default_ttl;
