-- +goose Up
CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    email text NOT NULL CHECK (email = lower(email) AND length(email) BETWEEN 3 AND 254),
    login text NOT NULL CHECK (login ~ '^[a-z0-9][a-z0-9_-]{1,31}$'),
    name text NOT NULL CHECK (length(name) BETWEEN 1 AND 128),
    password_hash text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX users_email_key ON users (lower(email));
CREATE UNIQUE INDEX users_login_key ON users (login);

-- +goose Down
DROP TABLE users;
