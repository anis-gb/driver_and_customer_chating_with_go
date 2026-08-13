-- Drop old tables if they exist to avoid conflicts
DROP TABLE IF EXISTS messages CASCADE;
DROP TABLE IF EXISTS conversations CASCADE;
DROP TABLE IF EXISTS users CASCADE;

-- 1. Create Tables
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    role VARCHAR(20) CHECK (role IN ('ADMIN', 'CUSTOMER', 'DRIVER')) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    admin_id UUID REFERENCES users(id) ON DELETE SET NULL,
    sended_by VARCHAR(20) CHECK (sended_by IN ('ADMIN', 'CUSTOMER', 'DRIVER')) NOT NULL,
    content TEXT NOT NULL,
    seen BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Index for fast history retrieval
CREATE INDEX idx_messages_user_created ON messages (user_id, created_at DESC);

-- 2. Insert Dummy Data
INSERT INTO users (id, name, role) VALUES 
('11111111-1111-1111-1111-111111111111', 'Admin Alice', 'ADMIN'),
('22222222-2222-2222-2222-222222222222', 'Admin Bob', 'ADMIN'),
('33333333-3333-3333-3333-333333333333', 'Customer Charlie', 'CUSTOMER'),
('44444444-4444-4444-4444-444444444444', 'Driver Dave', 'DRIVER');
