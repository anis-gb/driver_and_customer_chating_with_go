-- Drop old tables if they exist to avoid conflicts
DROP TABLE IF EXISTS messages CASCADE;
DROP TABLE IF EXISTS conversations CASCADE;
DROP TABLE IF EXISTS customer_messages CASCADE;

-- 1. Create Tables
CREATE TABLE customer_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id VARCHAR(255) NOT NULL,
    admin_id VARCHAR(255),
    sended_by VARCHAR(20) CHECK (sended_by IN ('ADMIN', 'CUSTOMER', 'DRIVER')) NOT NULL,
    content TEXT NOT NULL,
    seen BOOLEAN DEFAULT FALSE,
    voice_messages TEXT,
    photo TEXT,
    file TEXT,
    full_name VARCHAR(255),
    profile_picture TEXT,
    gender VARCHAR(10),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Index for fast history retrieval
CREATE INDEX idx_customer_messages_user_created ON customer_messages (user_id, created_at DESC);
