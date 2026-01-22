-- Clingy API Database Schema
-- Run this migration on the mvchat database
-- This is the consolidated schema including all migrations

-- Schema version tracking for auto-migrations
CREATE TABLE IF NOT EXISTS clingy_schema_version (
    version INT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Insert current version (matches this file's version number)
INSERT INTO clingy_schema_version (version) VALUES (1) ON CONFLICT DO NOTHING;

-- Pregnancies table
CREATE TABLE IF NOT EXISTS clingy_pregnancies (
    id BIGSERIAL PRIMARY KEY,
    owner_id TEXT NOT NULL UNIQUE,             -- mvchat user ID (mother) - UUID format
    partner_id TEXT,                           -- mvchat user ID (father) - UUID format
    partner_status VARCHAR(20),                -- NULL, 'pending', 'approved', 'denied'
    partner_permission VARCHAR(20),            -- NULL, 'read', 'write'
    partner_name VARCHAR(100),                 -- Display name for partner

    -- Pregnancy data
    due_date DATE,
    start_date DATE,
    calculation_method VARCHAR(20),            -- 'lmp', 'conception', 'due_date'
    cycle_length INT DEFAULT 28,
    baby_name VARCHAR(100),
    mom_name VARCHAR(100),
    mom_birthday DATE,                         -- Mom's birthday for age calculation
    gender VARCHAR(20),                        -- 'boy', 'girl', 'unsure'
    parent_role VARCHAR(20),                   -- 'mother', 'father'
    profile_photo TEXT,                        -- Server URL after upload

    -- Sharing visibility
    display_partner_card BOOLEAN DEFAULT TRUE, -- Hide admin partners from UI

    -- Co-owner for admin access (doesn't occupy partner slot)
    coowner_id TEXT,                           -- UUID of coowner (e.g., tsrlegends@gmail.com)
    coowner_name VARCHAR(100),                 -- Display name for coowner

    -- Outcome tracking
    outcome VARCHAR(20) DEFAULT 'ongoing',     -- ongoing, birth, miscarriage, ectopic, stillbirth
    outcome_date DATE,

    -- Archive functionality
    archived BOOLEAN DEFAULT FALSE,
    archived_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_clingy_pregnancies_owner ON clingy_pregnancies(owner_id);
CREATE INDEX IF NOT EXISTS idx_clingy_pregnancies_partner ON clingy_pregnancies(partner_id);
CREATE INDEX IF NOT EXISTS idx_clingy_pregnancies_coowner ON clingy_pregnancies(coowner_id);
CREATE INDEX IF NOT EXISTS idx_pregnancies_owner_archived ON clingy_pregnancies(owner_id, archived);

-- Entries table (generic table for all entry types)
CREATE TABLE IF NOT EXISTS clingy_entries (
    id BIGSERIAL PRIMARY KEY,
    pregnancy_id BIGINT NOT NULL REFERENCES clingy_pregnancies(id) ON DELETE CASCADE,
    client_id VARCHAR(50) NOT NULL,            -- Original client-side ID (timestamp)
    entry_type VARCHAR(50) NOT NULL,           -- 'weight', 'symptom', 'appointment', etc.
    data JSONB NOT NULL,                       -- Entry data as JSON
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,                    -- Soft delete for sync

    UNIQUE(pregnancy_id, entry_type, client_id)
);

CREATE INDEX IF NOT EXISTS idx_clingy_entries_pregnancy ON clingy_entries(pregnancy_id);
CREATE INDEX IF NOT EXISTS idx_clingy_entries_type ON clingy_entries(entry_type);
CREATE INDEX IF NOT EXISTS idx_clingy_entries_updated ON clingy_entries(updated_at);
CREATE INDEX IF NOT EXISTS idx_clingy_entries_deleted ON clingy_entries(deleted_at) WHERE deleted_at IS NOT NULL;

-- Settings table
CREATE TABLE IF NOT EXISTS clingy_settings (
    id BIGSERIAL PRIMARY KEY,
    pregnancy_id BIGINT NOT NULL REFERENCES clingy_pregnancies(id) ON DELETE CASCADE,
    setting_type VARCHAR(50) NOT NULL,         -- 'weight_settings', etc.
    data JSONB NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT NOW(),

    UNIQUE(pregnancy_id, setting_type)
);

-- Pairing requests table
CREATE TABLE IF NOT EXISTS clingy_pairing_requests (
    id BIGSERIAL PRIMARY KEY,
    requester_id TEXT NOT NULL,                -- Father's mvchat user ID - UUID format
    requester_name VARCHAR(100),               -- Display name for notification
    target_email VARCHAR(255) NOT NULL,        -- Mother's email
    target_id TEXT,                            -- Resolved mother's user ID - UUID format
    status VARCHAR(20) DEFAULT 'pending',      -- 'pending', 'approved', 'denied', 'cancelled'
    permission VARCHAR(20),                    -- 'read', 'write' (set on approval)
    created_at TIMESTAMPTZ DEFAULT NOW(),
    resolved_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_clingy_pairing_target ON clingy_pairing_requests(target_id);
CREATE INDEX IF NOT EXISTS idx_clingy_pairing_status ON clingy_pairing_requests(status);

-- Files table
CREATE TABLE IF NOT EXISTS clingy_files (
    id BIGSERIAL PRIMARY KEY,
    pregnancy_id BIGINT NOT NULL REFERENCES clingy_pregnancies(id) ON DELETE CASCADE,
    client_id VARCHAR(50),                     -- Original client-side ID
    file_type VARCHAR(50) NOT NULL,            -- 'profile_photo', 'photo_entry', 'medical_photo'
    storage_path TEXT NOT NULL,                -- Path in storage (e.g., uploads/clingy/...)
    mime_type VARCHAR(100),
    size_bytes BIGINT,
    metadata JSONB,                            -- category, week, caption, etc.
    created_at TIMESTAMPTZ DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_clingy_files_pregnancy ON clingy_files(pregnancy_id);

-- Sync state table
CREATE TABLE IF NOT EXISTS clingy_sync_state (
    id BIGSERIAL PRIMARY KEY,
    user_id TEXT NOT NULL,                     -- UUID format
    device_id VARCHAR(100) NOT NULL,
    last_sync_at TIMESTAMPTZ,
    last_sync_version BIGINT DEFAULT 0,

    UNIQUE(user_id, device_id)
);

-- Invite codes table
CREATE TABLE IF NOT EXISTS clingy_invite_codes (
    id BIGSERIAL PRIMARY KEY,
    pregnancy_id BIGINT NOT NULL REFERENCES clingy_pregnancies(id) ON DELETE CASCADE,
    code_hash VARCHAR(60) NOT NULL,            -- bcrypt hash of code
    code_prefix VARCHAR(4) NOT NULL,           -- First 4 chars for display (XXXX-****-**)
    role VARCHAR(20) NOT NULL,                 -- 'father' or 'support'
    permission VARCHAR(20) DEFAULT 'read',     -- 'read' or 'write' (for father)
    created_at TIMESTAMPTZ DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,           -- NOW() + 48 hours
    redeemed_at TIMESTAMPTZ,                   -- NULL until used
    redeemed_by TEXT,                          -- User ID who redeemed - UUID format
    revoked_at TIMESTAMPTZ,                    -- NULL unless manually revoked

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
    user_id TEXT NOT NULL,                     -- UUID format
    display_name VARCHAR(100),                 -- Name shown to mother
    permission VARCHAR(10) DEFAULT 'read',     -- 'read' or 'write' (for admin supporters)
    joined_at TIMESTAMPTZ DEFAULT NOW(),
    invited_via_code_id BIGINT REFERENCES clingy_invite_codes(id),
    removed_at TIMESTAMPTZ,                    -- Soft delete
    display_partner_card BOOLEAN DEFAULT TRUE, -- Hide admin supporters from UI

    UNIQUE(pregnancy_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_clingy_supporters_pregnancy ON clingy_supporters(pregnancy_id);
CREATE INDEX IF NOT EXISTS idx_clingy_supporters_user ON clingy_supporters(user_id);

-- Rate limiting for code redemption attempts
CREATE TABLE IF NOT EXISTS clingy_code_attempts (
    id BIGSERIAL PRIMARY KEY,
    user_id TEXT NOT NULL,                     -- UUID format
    attempted_at TIMESTAMPTZ DEFAULT NOW(),
    success BOOLEAN DEFAULT FALSE,
    ip_address VARCHAR(45)                     -- For additional rate limiting
);

CREATE INDEX IF NOT EXISTS idx_clingy_code_attempts_user ON clingy_code_attempts(user_id, attempted_at);
