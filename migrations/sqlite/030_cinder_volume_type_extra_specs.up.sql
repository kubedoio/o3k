-- Add extra_specs column to volume_types
ALTER TABLE volume_types ADD COLUMN extra_specs TEXT DEFAULT '{}';
