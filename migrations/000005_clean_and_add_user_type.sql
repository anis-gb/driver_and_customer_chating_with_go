-- Migration 000005: Clean contaminated test data and tag messages by user_type
-- This prevents driver messages from appearing in customer inbox and vice versa.

-- 1. Add user_type column to customer_messages (default 'CUSTOMER' for all existing rows)
ALTER TABLE customer_messages
    ADD COLUMN IF NOT EXISTS user_type VARCHAR(20) NOT NULL DEFAULT 'CUSTOMER'
        CHECK (user_type IN ('CUSTOMER', 'DRIVER'));

-- 2. Add user_type column to driver_messages (default 'DRIVER' for all existing rows)
ALTER TABLE driver_messages
    ADD COLUMN IF NOT EXISTS user_type VARCHAR(20) NOT NULL DEFAULT 'DRIVER'
        CHECK (user_type IN ('CUSTOMER', 'DRIVER'));

-- 3. Delete cross-contaminated rows:
--    Remove rows in customer_messages where user_type was set to DRIVER
--    (none yet, but cleanup in case of future bugs)
DELETE FROM customer_messages WHERE user_type = 'DRIVER';

-- Delete rows in driver_messages where user_type was set to CUSTOMER
DELETE FROM driver_messages WHERE user_type = 'CUSTOMER';

-- 4. Add indexes for performance
CREATE INDEX IF NOT EXISTS idx_customer_messages_user_type ON customer_messages(user_type);
CREATE INDEX IF NOT EXISTS idx_driver_messages_user_type ON driver_messages(user_type);
