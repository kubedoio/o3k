-- Add metadef namespaces table
CREATE TABLE IF NOT EXISTS metadef_namespaces (
    namespace VARCHAR(255) PRIMARY KEY,
    display_name VARCHAR(255),
    description TEXT,
    visibility VARCHAR(50) DEFAULT 'public',
    protected BOOLEAN DEFAULT FALSE,
    owner VARCHAR(255),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS metadef_resource_types (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    namespace VARCHAR(255) NOT NULL REFERENCES metadef_namespaces(namespace) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    prefix VARCHAR(50),
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(namespace, name)
);

CREATE INDEX idx_metadef_namespaces_visibility ON metadef_namespaces(visibility);
CREATE INDEX idx_metadef_resource_types_namespace ON metadef_resource_types(namespace);
