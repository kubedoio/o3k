-- Add locked column to instances table
ALTER TABLE instances ADD COLUMN locked INTEGER DEFAULT 0;
