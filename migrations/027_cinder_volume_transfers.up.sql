-- Add Cinder volume transfers table for ownership changes
CREATE TABLE IF NOT EXISTS volume_transfers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    volume_id UUID NOT NULL REFERENCES volumes(id) ON DELETE CASCADE,
    name VARCHAR(255),
    source_project_id UUID NOT NULL,
    destination_project_id UUID,
    auth_key VARCHAR(255) NOT NULL,
    accepted BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_volume_transfers_volume ON volume_transfers(volume_id);
CREATE INDEX idx_volume_transfers_source ON volume_transfers(source_project_id);
CREATE INDEX idx_volume_transfers_accepted ON volume_transfers(accepted);
