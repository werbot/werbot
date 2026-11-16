-- +goose Up
CREATE TABLE resource_network_rule (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    org_id uuid NOT NULL,
    resource_id uuid NOT NULL,
    cidr cidr NOT NULL,
    effect text NOT NULL CHECK (effect IN ('allow', 'deny')),
    UNIQUE (resource_id, cidr),
    FOREIGN KEY (resource_id, org_id) REFERENCES resource (id, org_id) ON DELETE CASCADE
);

CREATE TABLE resource_country_rule (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    org_id uuid NOT NULL,
    resource_id uuid NOT NULL,
    country char(2) NOT NULL CHECK (country ~ '^[A-Z]{2}$'),
    effect text NOT NULL CHECK (effect IN ('allow', 'deny')),
    UNIQUE (resource_id, country),
    FOREIGN KEY (resource_id, org_id) REFERENCES resource (id, org_id) ON DELETE CASCADE
);

CREATE TABLE resource_schedule (
    resource_id uuid NOT NULL,
    org_id uuid NOT NULL,
    weekday smallint NOT NULL CHECK (weekday BETWEEN 0 AND 6),
    hour smallint NOT NULL CHECK (hour BETWEEN 0 AND 23),
    PRIMARY KEY (resource_id, weekday, hour),
    FOREIGN KEY (resource_id, org_id) REFERENCES resource (id, org_id) ON DELETE CASCADE
);

-- +goose Down
DROP TABLE resource_schedule;
DROP TABLE resource_country_rule;
DROP TABLE resource_network_rule;
