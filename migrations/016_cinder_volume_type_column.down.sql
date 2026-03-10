-- Remove volume_type column from volumes table
ALTER TABLE volumes DROP COLUMN IF EXISTS volume_type;
