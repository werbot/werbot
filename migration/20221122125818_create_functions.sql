-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION wrb_add_host_key () RETURNS TRIGGER LANGUAGE plpgsql AS $function$
BEGIN
  IF TG_OP = 'INSERT' THEN
        INSERT INTO server_host_key(server_id) values (NEW.id);
        RETURN NEW;
    END IF;
END
$function$;

CREATE OR REPLACE FUNCTION wrb_updated_at () RETURNS TRIGGER LANGUAGE plpgsql AS $function$
BEGIN
	new.updated_at = NOW();
	RETURN NEW;
END;
$function$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP FUNCTION wrb_updated_at() CASCADE;
DROP FUNCTION wrb_add_host_key() CASCADE;
-- +goose StatementEnd
