-- Add mom's birthday field for age tracking
-- Run this migration on the mvchat database

ALTER TABLE clingy_pregnancies ADD COLUMN IF NOT EXISTS mom_birthday DATE;
