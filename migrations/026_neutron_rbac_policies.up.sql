-- Add Neutron RBAC policies table for resource sharing
CREATE TABLE IF NOT EXISTS rbac_policies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL,
    object_type VARCHAR(50) NOT NULL,
    object_id UUID NOT NULL,
    target_tenant UUID NOT NULL,
    action VARCHAR(50) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_rbac_policies_object ON rbac_policies(object_type, object_id);
CREATE INDEX idx_rbac_policies_target ON rbac_policies(target_tenant);
CREATE INDEX idx_rbac_policies_project ON rbac_policies(project_id);
