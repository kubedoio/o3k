PRAGMA foreign_keys = ON;

-- Add volume type access control for private volume types
CREATE TABLE IF NOT EXISTS volume_type_access (
    volume_type_id TEXT NOT NULL REFERENCES volume_types(id) ON DELETE CASCADE,
    project_id TEXT NOT NULL,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (volume_type_id, project_id)
);

CREATE INDEX idx_volume_type_access_type ON volume_type_access(volume_type_id);
CREATE INDEX idx_volume_type_access_project ON volume_type_access(project_id);

-- is_public column already exists on volume_types from the initial schema.
