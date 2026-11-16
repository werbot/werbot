-- +goose Up
CREATE TABLE access_binding (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    org_id uuid NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    subject_type text NOT NULL CHECK (subject_type IN ('user', 'team')),
    subject_id uuid NOT NULL,
    resource_id uuid NULL,
    label_selector jsonb NULL CHECK (label_selector IS NULL OR jsonb_typeof(label_selector) = 'object'),
    created_by uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (num_nonnulls(resource_id, label_selector) = 1),
    FOREIGN KEY (resource_id, org_id) REFERENCES resource (id, org_id) ON DELETE CASCADE
);
CREATE INDEX access_binding_org_idx ON access_binding (org_id);

-- subject_id is polymorphic (no FK): drop bindings with their subject so a
-- re-invited member or a recreated team never inherits a stale grant.
-- +goose StatementBegin
CREATE FUNCTION access_binding_drop_member() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    DELETE FROM access_binding
     WHERE org_id = OLD.org_id AND subject_type = 'user' AND subject_id = OLD.user_id;
    RETURN NULL;
END $$;

CREATE FUNCTION access_binding_drop_team() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    DELETE FROM access_binding
     WHERE org_id = OLD.org_id AND subject_type = 'team' AND subject_id = OLD.id;
    RETURN NULL;
END $$;
-- +goose StatementEnd

CREATE TRIGGER membership_drop_bindings AFTER DELETE ON membership
    FOR EACH ROW EXECUTE FUNCTION access_binding_drop_member();
CREATE TRIGGER team_drop_bindings AFTER DELETE ON team
    FOR EACH ROW EXECUTE FUNCTION access_binding_drop_team();

-- +goose Down
DROP TRIGGER team_drop_bindings ON team;
DROP TRIGGER membership_drop_bindings ON membership;
DROP FUNCTION access_binding_drop_team();
DROP FUNCTION access_binding_drop_member();
DROP TABLE access_binding;
