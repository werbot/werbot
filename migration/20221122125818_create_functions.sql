-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION wrb_scheme() RETURNS TRIGGER LANGUAGE plpgsql AS $function$
BEGIN
  IF TG_OP = 'INSERT' THEN
    IF (SELECT scheme_type FROM scheme WHERE id = NEW.id) IN (103) THEN
      INSERT INTO scheme_host_key (scheme_id)
      VALUES (NEW.id);
    END IF;

    INSERT INTO scheme_activity (scheme_id, data)
    VALUES (
      NEW.id,
      '{
        "mon":[1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1],
        "tue":[1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1],
        "wed":[1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1],
        "thu":[1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1],
        "fri":[1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1],
        "sat":[1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1],
        "sun":[1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1]
      }'
    );
    RETURN NEW;
  END IF;
END
$function$;

CREATE OR REPLACE FUNCTION wrb_updated_at() RETURNS TRIGGER LANGUAGE plpgsql AS $function$
BEGIN
	new.updated_at = NOW();
	RETURN NEW;
END;
$function$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP FUNCTION wrb_updated_at() CASCADE;
DROP FUNCTION wrb_scheme() CASCADE;
-- +goose StatementEnd
