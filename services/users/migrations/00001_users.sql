-- +goose Up
-- Banco próprio do users-service (database-per-service).
-- id é o "sub" do Keycloak. TEXT, não UUID: com federação (LDAP, Google...)
-- o sub não tem formato garantido.
CREATE TABLE users (
    id         TEXT PRIMARY KEY,
    name       TEXT        NOT NULL,
    email      TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

-- +goose Down
DROP TABLE users;
