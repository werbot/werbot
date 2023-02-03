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
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP FUNCTION "public".add_host_key() CASCADE;
-- +goose StatementEnd
