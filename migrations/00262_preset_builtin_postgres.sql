-- +goose Up
-- postgres becomes a builtin type and a preset name never equals a builtin
-- type: a preset named postgres becomes postgres-<first 8 hex digits of its
-- id>; its resources keep preset_id.
UPDATE resource_preset SET name = 'postgres-' || left(replace(id::text, '-', ''), 8) WHERE name = 'postgres';

-- +goose Down
-- The old name would collide with the builtin type again: Down keeps the new name.
