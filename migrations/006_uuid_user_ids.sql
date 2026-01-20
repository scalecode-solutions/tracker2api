-- Migration to support mvchat2 UUID user IDs
-- mvchat2 uses UUID format for user IDs instead of BIGINT
-- Run this migration on the mvchat database

-- Note: This migration assumes a fresh database or empty clingy tables
-- If migrating from old data, manual data conversion would be needed

-- clingy_pregnancies: owner_id and partner_id
ALTER TABLE clingy_pregnancies
    ALTER COLUMN owner_id TYPE TEXT,
    ALTER COLUMN partner_id TYPE TEXT;

-- Drop and recreate indexes with new types
DROP INDEX IF EXISTS idx_clingy_pregnancies_owner;
DROP INDEX IF EXISTS idx_clingy_pregnancies_partner;
CREATE INDEX idx_clingy_pregnancies_owner ON clingy_pregnancies(owner_id);
CREATE INDEX idx_clingy_pregnancies_partner ON clingy_pregnancies(partner_id);

-- clingy_pairing_requests: requester_id and target_id
ALTER TABLE clingy_pairing_requests
    ALTER COLUMN requester_id TYPE TEXT,
    ALTER COLUMN target_id TYPE TEXT;

DROP INDEX IF EXISTS idx_clingy_pairing_target;
CREATE INDEX idx_clingy_pairing_target ON clingy_pairing_requests(target_id);

-- clingy_invite_codes: redeemed_by
ALTER TABLE clingy_invite_codes
    ALTER COLUMN redeemed_by TYPE TEXT;

-- clingy_supporters: user_id
ALTER TABLE clingy_supporters
    ALTER COLUMN user_id TYPE TEXT;

DROP INDEX IF EXISTS idx_clingy_supporters_user;
CREATE INDEX idx_clingy_supporters_user ON clingy_supporters(user_id);

-- clingy_code_attempts: user_id
ALTER TABLE clingy_code_attempts
    ALTER COLUMN user_id TYPE TEXT;

DROP INDEX IF EXISTS idx_clingy_code_attempts_user;
CREATE INDEX idx_clingy_code_attempts_user ON clingy_code_attempts(user_id, attempted_at);

-- clingy_sync_state: user_id
ALTER TABLE clingy_sync_state
    ALTER COLUMN user_id TYPE TEXT;
