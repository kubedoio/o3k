-- Add locked column to instances table
ALTER TABLE instances ADD COLUMN IF NOT EXISTS locked BOOLEAN DEFAULT false;
