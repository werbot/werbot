-- +goose Up
-- +goose StatementBegin
CREATE TABLE "public"."update" (
  "id" uuid DEFAULT gen_random_uuid (),
  "component" varchar(10) NOT NULL,
  "version" varchar(10) NOT NULL,
  "version_after" varchar(10) NOT NULL,
  "description" varchar(255),
  "issued_at" timestamptz,
  PRIMARY KEY ("id")
);

INSERT INTO "public"."update" ("id", "component", "version", "version_after", "description", "issued_at") VALUES
('93904d0d-9706-4fb4-a455-5d015be915a6', 'avocado', '1.0', '1.0', '', 'NOW()'),
('5343aa48-7973-4b85-a1ef-70388e1a8727', 'buffet', '1.0', '1.0', '','NOW()'),
('89de9660-166d-40ea-bde3-3da3c797b142', 'chef', '1.1', '1.0', 'Message','NOW()'),
('1bfe3406-3f95-4059-86bd-ae2c44859c02', 'ghost', '1.0', '1.0', '','NOW()'),
('35451aaf-401e-4020-bdf9-18b1cd5eb373', 'taco', '1.0', '1.0', '','NOW()');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS "public"."update";
-- +goose StatementEnd
