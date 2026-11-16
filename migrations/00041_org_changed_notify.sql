-- +goose Up
-- +goose StatementBegin
CREATE FUNCTION werbot_notify_org_changed() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_TABLE_NAME = 'ip_blocklist' THEN
        PERFORM pg_notify('werbot_org_changed', '*');
    ELSIF TG_OP = 'DELETE' THEN
        PERFORM pg_notify('werbot_org_changed', OLD.org_id::text);
    ELSE
        PERFORM pg_notify('werbot_org_changed', NEW.org_id::text);
    END IF;
    RETURN NULL;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER resource_org_changed AFTER INSERT OR UPDATE OR DELETE ON resource FOR EACH ROW EXECUTE FUNCTION werbot_notify_org_changed();
CREATE TRIGGER resource_label_org_changed AFTER INSERT OR UPDATE OR DELETE ON resource_label FOR EACH ROW EXECUTE FUNCTION werbot_notify_org_changed();
CREATE TRIGGER access_binding_org_changed AFTER INSERT OR UPDATE OR DELETE ON access_binding FOR EACH ROW EXECUTE FUNCTION werbot_notify_org_changed();
CREATE TRIGGER resource_network_rule_org_changed AFTER INSERT OR UPDATE OR DELETE ON resource_network_rule FOR EACH ROW EXECUTE FUNCTION werbot_notify_org_changed();
CREATE TRIGGER resource_country_rule_org_changed AFTER INSERT OR UPDATE OR DELETE ON resource_country_rule FOR EACH ROW EXECUTE FUNCTION werbot_notify_org_changed();
CREATE TRIGGER resource_schedule_org_changed AFTER INSERT OR UPDATE OR DELETE ON resource_schedule FOR EACH ROW EXECUTE FUNCTION werbot_notify_org_changed();
CREATE TRIGGER team_org_changed AFTER INSERT OR UPDATE OR DELETE ON team FOR EACH ROW EXECUTE FUNCTION werbot_notify_org_changed();
CREATE TRIGGER team_member_org_changed AFTER INSERT OR UPDATE OR DELETE ON team_member FOR EACH ROW EXECUTE FUNCTION werbot_notify_org_changed();
CREATE TRIGGER membership_org_changed AFTER INSERT OR UPDATE OR DELETE ON membership FOR EACH ROW EXECUTE FUNCTION werbot_notify_org_changed();
-- ip_blocklist is rewritten in bulk (firehol refresh): one notification per statement.
CREATE TRIGGER ip_blocklist_org_changed AFTER INSERT OR UPDATE OR DELETE ON ip_blocklist FOR EACH STATEMENT EXECUTE FUNCTION werbot_notify_org_changed();

-- +goose Down
DROP TRIGGER ip_blocklist_org_changed ON ip_blocklist;
DROP TRIGGER membership_org_changed ON membership;
DROP TRIGGER team_member_org_changed ON team_member;
DROP TRIGGER team_org_changed ON team;
DROP TRIGGER resource_schedule_org_changed ON resource_schedule;
DROP TRIGGER resource_country_rule_org_changed ON resource_country_rule;
DROP TRIGGER resource_network_rule_org_changed ON resource_network_rule;
DROP TRIGGER access_binding_org_changed ON access_binding;
DROP TRIGGER resource_label_org_changed ON resource_label;
DROP TRIGGER resource_org_changed ON resource;
DROP FUNCTION werbot_notify_org_changed();
