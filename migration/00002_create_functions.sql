-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION add_host_key()
  RETURNS "pg_catalog"."trigger" AS $BODY$
BEGIN
  IF    TG_OP = 'INSERT' THEN
        INSERT INTO server_host_key(server_id) values (NEW.id);
        RETURN NEW;
    END IF;
END
$BODY$
  LANGUAGE plpgsql VOLATILE
  COST 100;

CREATE OR REPLACE FUNCTION update_server_password()
  RETURNS "pg_catalog"."trigger" AS $BODY$ 
BEGIN
    CASE TG_OP
            WHEN 'INSERT' THEN
                IF NEW."password" IS NULL THEN
                    UPDATE "server" SET "password" = '' WHERE "id" = NEW.ID;
                END IF;
            WHEN 'UPDATE' THEN
                IF NEW."password" IS NULL THEN
                    UPDATE "server" SET "password" = '' WHERE "id" = ID;
                END IF;
    END CASE;
RETURN NULL;
END $BODY$
  LANGUAGE plpgsql VOLATILE
  COST 100;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP FUNCTION "public".update_server_password() CASCADE;
DROP FUNCTION "public".add_host_key() CASCADE;
-- +goose StatementEnd
