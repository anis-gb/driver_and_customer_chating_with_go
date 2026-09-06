-- Message history table for tracking edits (audit trail)
CREATE TABLE IF NOT EXISTS message_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    message_id UUID NOT NULL,
    message_type VARCHAR(20) NOT NULL CHECK (message_type IN ('CUSTOMER', 'DRIVER')),
    edited_by_user_id VARCHAR(255) NOT NULL,
    edited_by_user_name VARCHAR(255) NOT NULL,
    edited_by_user_type VARCHAR(20) NOT NULL CHECK (edited_by_user_type IN ('ADMIN', 'CUSTOMER', 'DRIVER')),
    edit_time TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    old_value TEXT,
    new_value TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_message_history_message_id ON message_history(message_id);
CREATE INDEX IF NOT EXISTS idx_message_history_edited_by ON message_history(edited_by_user_id);
CREATE INDEX IF NOT EXISTS idx_message_history_edit_time ON message_history(edit_time DESC);
