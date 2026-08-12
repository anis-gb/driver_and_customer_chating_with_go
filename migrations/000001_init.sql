-- Initial migration: extend this as your schema grows.
-- Run manually with psql, or wire up a migration tool such as
-- golang-migrate (https://github.com/golang-migrate/migrate).

CREATE TABLE IF NOT EXISTS users (
    id         SERIAL PRIMARY KEY,
    name       VARCHAR(255) NOT NULL,
    email      VARCHAR(255) NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
