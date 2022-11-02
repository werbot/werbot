-- +goose Up
-- +goose StatementBegin
CREATE TABLE "public"."license_type" (
  "id" uuid DEFAULT gen_random_uuid (),
  "name" varchar(64) NOT NULL,
  "period" varchar(64) NOT NULL,
  "default" bool NOT NULL DEFAULT false,
  "companies" int4 NOT NULL,
  "servers" int4 NOT NULL,
  "users" int4 NOT NULL,
  "modules" json,
  PRIMARY KEY ("id")
);

CREATE TABLE "public"."license" (
  "id" uuid DEFAULT gen_random_uuid (),
  "version" int4 NOT NULL,
  "customer_id" uuid,
  "subscriber_id" uuid,
  "license_type_id" uuid NOT NULL,
  "ip" inet,
  "status" varchar(64) NOT NULL,
  "issued_at" timestamptz,
  "expires_at" timestamptz,
  "license" bytea,
  FOREIGN KEY ("license_type_id") REFERENCES "public"."license_type" ("id") ON DELETE CASCADE ON UPDATE NO ACTION,
  PRIMARY KEY ("id")
);

INSERT INTO "public"."license_type" ("id", "name", "period", "default", "companies", "servers", "users", "modules") VALUES 
('b61d3ab1-bd9d-4ee8-a103-2c11d1271d6a', 'Enterprise trial', '14', true, 5, 200, 20, '["success", "error", "warning"]');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS "public"."license";
DROP TABLE IF EXISTS "public"."license_type";
-- +goose StatementEnd
