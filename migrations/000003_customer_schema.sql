
-- 1. Create Tables
CREATE TABLE customer_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id VARCHAR(255) NOT NULL,
    user_mobile VARCHAR(255),
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

-- Add index for faster queries
CREATE INDEX idx_customer_messages_user_id ON customer_messages(user_id);
CREATE INDEX idx_customer_messages_user_mobile ON customer_messages(user_mobile);
CREATE INDEX idx_customer_messages_created_at ON customer_messages(created_at DESC);

