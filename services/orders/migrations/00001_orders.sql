-- +goose Up
-- Banco próprio do orders-service (database-per-service). user_id aponta
-- para um usuário de OUTRO serviço: não há (nem pode haver) foreign key.
CREATE TABLE orders (
    id          UUID PRIMARY KEY,
    user_id     TEXT        NOT NULL,
    total_cents BIGINT      NOT NULL CHECK (total_cents > 0),
    created_at  TIMESTAMPTZ NOT NULL
);
CREATE INDEX orders_user_id_idx ON orders (user_id);

-- +goose Down
DROP TABLE orders;
