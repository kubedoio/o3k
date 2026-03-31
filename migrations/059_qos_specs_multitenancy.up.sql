-- Add project_id column for multi-tenancy support
ALTER TABLE qos_specs ADD COLUMN IF NOT EXISTS project_id UUID NOT NULL DEFAULT '00000000-0000-0000-0000-000000000002';

-- Drop old unique constraint on name
ALTER TABLE qos_specs DROP CONSTRAINT IF EXISTS qos_specs_name_key;

-- Add new unique constraint on (name, project_id)
ALTER TABLE qos_specs ADD CONSTRAINT qos_specs_name_project_unique UNIQUE (name, project_id);

-- Add index on project_id for filtering
CREATE INDEX IF NOT EXISTS idx_qos_specs_project_id ON qos_specs(project_id);

-- Add updated_at column
ALTER TABLE qos_specs ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP NOT NULL DEFAULT NOW();
