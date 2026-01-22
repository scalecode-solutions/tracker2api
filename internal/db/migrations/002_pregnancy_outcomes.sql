-- Migration: Add pregnancy outcome and archive functionality
-- Date: 2026-01-12

-- Add outcome tracking
ALTER TABLE clingy_pregnancies ADD COLUMN IF NOT EXISTS outcome VARCHAR(20) DEFAULT 'ongoing';
ALTER TABLE clingy_pregnancies ADD COLUMN IF NOT EXISTS outcome_date DATE;

-- Add archive functionality
ALTER TABLE clingy_pregnancies ADD COLUMN IF NOT EXISTS archived BOOLEAN DEFAULT false;
ALTER TABLE clingy_pregnancies ADD COLUMN IF NOT EXISTS archived_at TIMESTAMP WITH TIME ZONE;

-- Index for efficient queries
CREATE INDEX IF NOT EXISTS idx_pregnancies_owner_archived ON clingy_pregnancies(owner_id, archived);

-- Add comments
COMMENT ON COLUMN clingy_pregnancies.outcome IS 'ongoing, birth, miscarriage, ectopic, stillbirth';
COMMENT ON COLUMN clingy_pregnancies.archived IS 'User-controlled archive status. Archived = read-only';
