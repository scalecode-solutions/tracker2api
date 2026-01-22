-- Add permission column to supporters table and coowner_id to pregnancies
-- Run this migration on the mvchat database

-- Add permission to supporters (for regular supporters)
ALTER TABLE clingy_supporters ADD COLUMN IF NOT EXISTS permission VARCHAR(10) DEFAULT 'read';

-- Add coowner_id to pregnancies (for admin access without occupying partner slot)
ALTER TABLE clingy_pregnancies ADD COLUMN IF NOT EXISTS coowner_id VARCHAR(255);
ALTER TABLE clingy_pregnancies ADD COLUMN IF NOT EXISTS coowner_name VARCHAR(255);
