-- Add flavor access table for tenant/project-specific flavor visibility
CREATE TABLE IF NOT EXISTS flavor_access (
    flavor_id UUID NOT NULL REFERENCES flavors(id) ON DELETE CASCADE,
    project_id UUID NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    PRIMARY KEY (flavor_id, project_id)
);

CREATE INDEX idx_flavor_access_project ON flavor_access(project_id);
