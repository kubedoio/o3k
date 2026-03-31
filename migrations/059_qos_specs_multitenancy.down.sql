-- Remove multi-tenancy support
ALTER TABLE qos_specs DROP COLUMN IF EXISTS project_id;
ALTER TABLE qos_specs DROP COLUMN IF EXISTS updated_at;
ALTER TABLE qos_specs DROP CONSTRAINT IF EXISTS qos_specs_name_project_unique;
ALTER TABLE qos_specs ADD CONSTRAINT qos_specs_name_key UNIQUE (name);
DROP INDEX IF EXISTS idx_qos_specs_project_id;
