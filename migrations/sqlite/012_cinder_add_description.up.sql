-- Add description column to volumes and snapshots tables
ALTER TABLE volumes ADD COLUMN description TEXT;
ALTER TABLE snapshots ADD COLUMN description TEXT;
