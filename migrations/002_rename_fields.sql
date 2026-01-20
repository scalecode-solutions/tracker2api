-- Migration 002: Rename fields to match Clingy app
-- Run this on the deployed database to rename columns

-- Rename baby_gender to gender
ALTER TABLE clingy_pregnancies RENAME COLUMN baby_gender TO gender;

-- Rename profile_photo_url to profile_photo
ALTER TABLE clingy_pregnancies RENAME COLUMN profile_photo_url TO profile_photo;
