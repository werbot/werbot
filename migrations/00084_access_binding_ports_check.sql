-- +goose Up
-- At most 16 non-empty port ranges within 1-65535; '{}' means every port.
-- Overlapping ranges are rejected by the application (connector.ValidatePorts).
ALTER TABLE access_binding ADD CONSTRAINT access_binding_ports_check CHECK (
    cardinality(ports) <= 16
    AND array_position(ports, NULL) IS NULL
    AND NOT ('empty'::int4range = ANY (ports))
    AND int4range(1, 65535, '[]') @> ALL (ports));

-- +goose Down
ALTER TABLE access_binding DROP CONSTRAINT access_binding_ports_check;
