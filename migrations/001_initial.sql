-- Clingy API Database Schema
-- Run this migration on the mvchat database

-- Pregnancies table
CREATE TABLE IF NOT EXISTS clingy_pregnancies (
    id BIGSERIAL PRIMARY KEY,
    owner_id BIGINT NOT NULL UNIQUE,      -- mvchat user ID (mother)
    partner_id BIGINT,                     -- mvchat user ID (father)
    partner_status VARCHAR(20),            -- NULL, 'pending', 'approved', 'denied'
    partner_permission VARCHAR(20),        -- NULL, 'read', 'write'

    -- Pregnancy data
    due_date DATE,
    start_date DATE,
    calculation_method VARCHAR(20),        -- 'lmp', 'conception', 'due_date'
    cycle_length INT DEFAULT 28,
    baby_name VARCHAR(100),
    mom_name VARCHAR(100),
    gender VARCHAR(20),                    -- 'boy', 'girl', 'unsure'
    parent_role VARCHAR(20),               -- 'mother', 'father'
    profile_photo TEXT,                    -- Server URL after upload

    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_clingy_pregnancies_owner ON clingy_pregnancies(owner_id);
CREATE INDEX IF NOT EXISTS idx_clingy_pregnancies_partner ON clingy_pregnancies(partner_id);

-- Entries table (generic table for all entry types)
CREATE TABLE IF NOT EXISTS clingy_entries (
    id BIGSERIAL PRIMARY KEY,
    pregnancy_id BIGINT NOT NULL REFERENCES clingy_pregnancies(id) ON DELETE CASCADE,
    client_id VARCHAR(50) NOT NULL,        -- Original client-side ID (timestamp)
    entry_type VARCHAR(50) NOT NULL,       -- 'weight', 'symptom', 'appointment', etc.
    data JSONB NOT NULL,                   -- Entry data as JSON
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,                -- Soft delete for sync

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
    setting_type VARCHAR(50) NOT NULL,     -- 'weight_settings', etc.
    data JSONB NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT NOW(),

    UNIQUE(pregnancy_id, setting_type)
);

-- Pairing requests table
CREATE TABLE IF NOT EXISTS clingy_pairing_requests (
    id BIGSERIAL PRIMARY KEY,
    requester_id BIGINT NOT NULL,          -- Father's mvchat user ID
    requester_name VARCHAR(100),           -- Display name for notification
    target_email VARCHAR(255) NOT NULL,    -- Mother's email
    target_id BIGINT,                      -- Resolved mother's user ID
    status VARCHAR(20) DEFAULT 'pending',  -- 'pending', 'approved', 'denied', 'cancelled'
    permission VARCHAR(20),                -- 'read', 'write' (set on approval)
    created_at TIMESTAMPTZ DEFAULT NOW(),
    resolved_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_clingy_pairing_target ON clingy_pairing_requests(target_id);
CREATE INDEX IF NOT EXISTS idx_clingy_pairing_status ON clingy_pairing_requests(status);

-- Files table
CREATE TABLE IF NOT EXISTS clingy_files (
    id BIGSERIAL PRIMARY KEY,
    pregnancy_id BIGINT NOT NULL REFERENCES clingy_pregnancies(id) ON DELETE CASCADE,
    client_id VARCHAR(50),                 -- Original client-side ID
    file_type VARCHAR(50) NOT NULL,        -- 'profile_photo', 'photo_entry', 'medical_photo'
    storage_path TEXT NOT NULL,            -- Path in storage (e.g., uploads/clingy/...)
    mime_type VARCHAR(100),
    size_bytes BIGINT,
    metadata JSONB,                        -- category, week, caption, etc.
    created_at TIMESTAMPTZ DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_clingy_files_pregnancy ON clingy_files(pregnancy_id);

-- Sync state table
CREATE TABLE IF NOT EXISTS clingy_sync_state (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    device_id VARCHAR(100) NOT NULL,
    last_sync_at TIMESTAMPTZ,
    last_sync_version BIGINT DEFAULT 0,

    UNIQUE(user_id, device_id)
);
