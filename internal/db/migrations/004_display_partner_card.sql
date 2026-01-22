-- Add display_partner_card flag to hide admin partners from UI
-- Run this migration on the mvchat database

-- Add to pregnancies table (for father)
ALTER TABLE clingy_pregnancies ADD COLUMN IF NOT EXISTS display_partner_card BOOLEAN DEFAULT TRUE;

-- Add to supporters table (for support users)
ALTER TABLE clingy_supporters ADD COLUMN IF NOT EXISTS display_partner_card BOOLEAN DEFAULT TRUE;
