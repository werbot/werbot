-- +goose Up
-- +goose StatementBegin
-- Scheme activity grids must hold 24 hourly values (00:00-23:00). Older rows
-- could only be written with 23 values because the protobuf validation
-- required exactly 23 items. Pad such rows with a denied trailing hour so the
-- migration never widens access; fail-closed is the safe default.
UPDATE "scheme_activity" AS sa
SET "data" = (
  SELECT jsonb_object_agg(
    day.key,
    CASE
      WHEN jsonb_typeof(day.value) = 'array' AND jsonb_array_length(day.value) = 23
        THEN day.value || '0'::jsonb
      ELSE day.value
    END
  )
  FROM jsonb_each(sa."data") AS day
)
WHERE jsonb_typeof(sa."data") = 'object'
  AND EXISTS (
    SELECT 1
    FROM jsonb_each(sa."data") AS day
    WHERE jsonb_typeof(day.value) = 'array' AND jsonb_array_length(day.value) = 23
  );
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- The padding is not reversible: a 24 hour row cannot be distinguished from a
-- row that legitimately allowed hour 23 before the padding was applied.
SELECT 1;
-- +goose StatementEnd
