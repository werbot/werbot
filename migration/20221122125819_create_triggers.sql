-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE TRIGGER "t_host_key" AFTER INSERT ON "public"."server" FOR EACH ROW EXECUTE PROCEDURE "public"."add_host_key"();
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER t_host_key ON "server";
-- +goose StatementEnd
