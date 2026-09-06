-- Bot status history table for tracking AI bot enable/disable events (audit trail)
CREATE TABLE IF NOT EXISTS bot_status_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    target_user_id VARCHAR(255) NOT NULL,
    user_role VARCHAR(20) NOT NULL CHECK (user_role IN ('CUSTOMER', 'DRIVER')),
    enabled BOOLEAN NOT NULL,
    changed_by_user_id VARCHAR(255) NOT NULL,
    changed_by_user_name VARCHAR(255) NOT NULL,
    changed_by_user_type VARCHAR(20) NOT NULL CHECK (changed_by_user_type IN ('ADMIN', 'SYSTEM')),
    reason VARCHAR(100) DEFAULT 'MANUAL_TOGGLE',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_bot_status_history_target ON bot_status_history(target_user_id, user_role);
CREATE INDEX IF NOT EXISTS idx_bot_status_history_changed_by ON bot_status_history(changed_by_user_id);
CREATE INDEX IF NOT EXISTS idx_bot_status_history_created_at ON bot_status_history(created_at DESC);
