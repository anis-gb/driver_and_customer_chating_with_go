-- Migration 000002: messages table
-- Run with:
--   psql -U postgres -d chat_api -f migrations/000002_create_messages.sql

CREATE TABLE IF NOT EXISTS messages (
    id         BIGSERIAL    PRIMARY KEY,
    sender_id  BIGINT       NOT NULL,
    content    TEXT         NOT NULL,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT now()
);
