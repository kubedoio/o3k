-- Remove description column from volumes and snapshots tables
ALTER TABLE volumes DROP COLUMN IF EXISTS description;
ALTER TABLE snapshots DROP COLUMN IF EXISTS description;
