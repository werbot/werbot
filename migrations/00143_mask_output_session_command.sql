-- +goose Up
ALTER TABLE org_setting ADD COLUMN mask_output text NOT NULL DEFAULT 'agents' CHECK (mask_output IN ('agents', 'all', 'none'));
ALTER TABLE session ADD COLUMN command text, ADD COLUMN classes text[];

-- +goose Down
ALTER TABLE session DROP COLUMN classes, DROP COLUMN command;
ALTER TABLE org_setting DROP COLUMN mask_output;
