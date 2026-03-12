CREATE TABLE IF NOT EXISTS qos_specs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE,
    consumer VARCHAR(50) NOT NULL DEFAULT 'back-end',
    specs JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_qos_specs_name ON qos_specs(name);
