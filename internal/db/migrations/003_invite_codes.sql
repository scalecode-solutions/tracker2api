-- Invite codes for partner/supporter sharing
-- Run this migration on the mvchat database

-- Invite codes table
CREATE TABLE IF NOT EXISTS clingy_invite_codes (
    id BIGSERIAL PRIMARY KEY,
    pregnancy_id BIGINT NOT NULL REFERENCES clingy_pregnancies(id) ON DELETE CASCADE,
    code_hash VARCHAR(60) NOT NULL,           -- bcrypt hash of code
    code_prefix VARCHAR(4) NOT NULL,          -- First 4 chars for display (XXXX-****-**)
    role VARCHAR(20) NOT NULL,                -- 'father' or 'support'
    permission VARCHAR(20) DEFAULT 'read',    -- 'read' or 'write' (for father)
    created_at TIMESTAMPTZ DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,          -- NOW() + 48 hours
    redeemed_at TIMESTAMPTZ,                  -- NULL until used
    redeemed_by BIGINT,                       -- User ID who redeemed
    revoked_at TIMESTAMPTZ,                   -- NULL unless manually revoked

    CONSTRAINT valid_invite_role CHECK (role IN ('father', 'support')),
    CONSTRAINT valid_invite_permission CHECK (permission IN ('read', 'write'))
);

CREATE INDEX IF NOT EXISTS idx_clingy_invite_codes_pregnancy ON clingy_invite_codes(pregnancy_id);
CREATE INDEX IF NOT EXISTS idx_clingy_invite_codes_active ON clingy_invite_codes(expires_at)
    WHERE redeemed_at IS NULL AND revoked_at IS NULL;

-- Supporters table (many supporters per pregnancy)
CREATE TABLE IF NOT EXISTS clingy_supporters (
    id BIGSERIAL PRIMARY KEY,
    pregnancy_id BIGINT NOT NULL REFERENCES clingy_pregnancies(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL,
    display_name VARCHAR(100),                -- Name shown to mother
    joined_at TIMESTAMPTZ DEFAULT NOW(),
    invited_via_code_id BIGINT REFERENCES clingy_invite_codes(id),
    removed_at TIMESTAMPTZ,                   -- Soft delete

    UNIQUE(pregnancy_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_clingy_supporters_pregnancy ON clingy_supporters(pregnancy_id);
CREATE INDEX IF NOT EXISTS idx_clingy_supporters_user ON clingy_supporters(user_id);

-- Rate limiting for code redemption attempts
CREATE TABLE IF NOT EXISTS clingy_code_attempts (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    attempted_at TIMESTAMPTZ DEFAULT NOW(),
    success BOOLEAN DEFAULT FALSE,
    ip_address VARCHAR(45)                    -- For additional rate limiting
);

CREATE INDEX IF NOT EXISTS idx_clingy_code_attempts_user ON clingy_code_attempts(user_id, attempted_at);

-- Add partner name to pregnancies table
ALTER TABLE clingy_pregnancies ADD COLUMN IF NOT EXISTS partner_name VARCHAR(100);
