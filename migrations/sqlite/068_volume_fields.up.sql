-- Add availability_zone and encrypted columns to volumes table
ALTER TABLE volumes
ADD COLUMN availability_zone TEXT DEFAULT 'nova';

ALTER TABLE volumes
ADD COLUMN encrypted INTEGER DEFAULT 0;
