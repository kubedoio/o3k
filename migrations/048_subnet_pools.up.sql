CREATE TABLE IF NOT EXISTS subnet_pools (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    prefixes TEXT[] NOT NULL,
    min_prefixlen INTEGER NOT NULL DEFAULT 8,
    max_prefixlen INTEGER NOT NULL DEFAULT 32,
    default_prefixlen INTEGER,
    shared BOOLEAN NOT NULL DEFAULT FALSE,
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    ip_version INTEGER NOT NULL DEFAULT 4,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_subnet_pools_project_id ON subnet_pools(project_id);
CREATE INDEX IF NOT EXISTS idx_subnet_pools_shared ON subnet_pools(shared) WHERE shared = TRUE;
CREATE INDEX IF NOT EXISTS idx_subnet_pools_is_default ON subnet_pools(is_default) WHERE is_default = TRUE;
