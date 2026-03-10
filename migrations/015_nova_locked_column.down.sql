-- Remove locked column from instances table
ALTER TABLE instances DROP COLUMN IF EXISTS locked;
