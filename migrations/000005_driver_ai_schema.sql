-- 5. Create driver_ai_settings table
CREATE TABLE IF NOT EXISTS driver_ai_settings (
    user_id VARCHAR(255) PRIMARY KEY,
    ai_enabled BOOLEAN DEFAULT TRUE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
