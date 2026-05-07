CREATE TABLE IF NOT EXISTS keystone_policies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type VARCHAR(50) NOT NULL DEFAULT 'application/json',
    blob TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Default policy rules
INSERT INTO keystone_policies (type, blob) VALUES (
    'application/json',
    '{
        "admin_required": "role:admin",
        "owner": "user_id:%(target.user_id)s",
        "admin_or_owner": "rule:admin_required or rule:owner",
        "compute:create": "role:member or role:admin",
        "compute:get": "rule:admin_or_owner",
        "compute:delete": "rule:admin_or_owner",
        "network:create_network": "role:member or role:admin",
        "network:delete_network": "rule:admin_or_owner",
        "volume:create": "role:member or role:admin",
        "volume:delete": "rule:admin_or_owner",
        "image:upload_image": "role:member or role:admin",
        "image:delete_image": "rule:admin_or_owner"
    }'
);
