-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE TRIGGER "tg_scheme" AFTER INSERT ON "scheme" FOR EACH ROW EXECUTE FUNCTION wrb_scheme();

DO $$
DECLARE
    table_name TEXT;
    tables TEXT[] := ARRAY[
        'profile',
        'project',
        'project_ldap',
        'project_member',
        'scheme',
        'scheme_member',
        'scheme_activity',
        'profile_public_key',
        'scheme_host_key',
        'project_api',
        'firewall_list',
        'token'
    ];
BEGIN
    FOREACH table_name IN ARRAY tables
    LOOP
        EXECUTE format('
            CREATE OR REPLACE TRIGGER tg_updated_at
            BEFORE UPDATE ON %I
            FOR EACH ROW
            EXECUTE FUNCTION wrb_updated_at();
        ', table_name);
    END LOOP;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$
DECLARE
    table_name TEXT;
    tables TEXT[] := ARRAY[
        'token',
        'profile',
        'project',
        'project_ldap',
        'project_member',
        'scheme',
        'scheme_member',
        'scheme_activity',
        'profile_public_key',
        'scheme_host_key',
        'project_api',
        'firewall_list'
    ];
BEGIN
    FOREACH table_name IN ARRAY tables
    LOOP
        EXECUTE format('
            DROP TRIGGER IF EXISTS tg_updated_at ON %I;
        ', table_name);
    END LOOP;
END $$;

DROP TRIGGER "tg_scheme" ON "scheme";
-- +goose StatementEnd
