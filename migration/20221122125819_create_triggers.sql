-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE TRIGGER "t_host_key" AFTER INSERT ON "public"."server" FOR EACH ROW EXECUTE PROCEDURE "public"."add_host_key"();
CREATE OR REPLACE TRIGGER "t_server_password" AFTER INSERT OR UPDATE ON "public"."server" FOR EACH ROW EXECUTE PROCEDURE "public"."update_server_password"();
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER t_host_key ON "server";
DROP TRIGGER t_server_password ON "server";
-- +goose StatementEnd
