-- Keystone tables
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS projects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) UNIQUE NOT NULL,
    description TEXT,
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS role_assignments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    project_id UUID REFERENCES projects(id) ON DELETE CASCADE,
    role_id UUID REFERENCES roles(id) ON DELETE CASCADE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, project_id, role_id)
);

-- Nova tables
CREATE TABLE IF NOT EXISTS flavors (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) UNIQUE NOT NULL,
    vcpus INTEGER NOT NULL,
    ram_mb INTEGER NOT NULL,
    disk_gb INTEGER NOT NULL,
    is_public BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS instances (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    project_id UUID REFERENCES projects(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    flavor_id UUID REFERENCES flavors(id),
    image_id UUID,
    status VARCHAR(50) NOT NULL DEFAULT 'BUILD',
    power_state INTEGER DEFAULT 0,
    libvirt_domain_id VARCHAR(255),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    launched_at TIMESTAMP,
    terminated_at TIMESTAMP
);

CREATE TABLE IF NOT EXISTS keypairs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    public_key TEXT NOT NULL,
    fingerprint VARCHAR(255),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, name)
);

-- Neutron tables
CREATE TABLE IF NOT EXISTS networks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    project_id UUID REFERENCES projects(id) ON DELETE CASCADE,
    admin_state_up BOOLEAN DEFAULT true,
    status VARCHAR(50) DEFAULT 'ACTIVE',
    shared BOOLEAN DEFAULT false,
    mtu INTEGER DEFAULT 1500,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS subnets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    network_id UUID REFERENCES networks(id) ON DELETE CASCADE,
    project_id UUID REFERENCES projects(id) ON DELETE CASCADE,
    cidr VARCHAR(50) NOT NULL,
    gateway_ip VARCHAR(50),
    ip_version INTEGER DEFAULT 4,
    enable_dhcp BOOLEAN DEFAULT true,
    dns_nameservers TEXT[],
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS ports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255),
    network_id UUID REFERENCES networks(id) ON DELETE CASCADE,
    project_id UUID REFERENCES projects(id) ON DELETE CASCADE,
    device_id UUID,
    device_owner VARCHAR(255),
    mac_address VARCHAR(17),
    admin_state_up BOOLEAN DEFAULT true,
    status VARCHAR(50) DEFAULT 'DOWN',
    fixed_ips JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS security_groups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    project_id UUID REFERENCES projects(id) ON DELETE CASCADE,
    description TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(project_id, name)
);

CREATE TABLE IF NOT EXISTS security_group_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    security_group_id UUID REFERENCES security_groups(id) ON DELETE CASCADE,
    direction VARCHAR(10) NOT NULL, -- 'ingress' or 'egress'
    ethertype VARCHAR(10) DEFAULT 'IPv4',
    protocol VARCHAR(10), -- 'tcp', 'udp', 'icmp', or NULL for all
    port_range_min INTEGER,
    port_range_max INTEGER,
    remote_ip_prefix VARCHAR(50),
    remote_group_id UUID REFERENCES security_groups(id) ON DELETE CASCADE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Cinder tables
CREATE TABLE IF NOT EXISTS volumes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255),
    project_id UUID REFERENCES projects(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    size_gb INTEGER NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'creating',
    bootable BOOLEAN DEFAULT false,
    attached_to_instance_id UUID REFERENCES instances(id) ON DELETE SET NULL,
    rbd_pool VARCHAR(255),
    rbd_image VARCHAR(255),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS volume_types (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) UNIQUE NOT NULL,
    description TEXT,
    is_public BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS snapshots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255),
    volume_id UUID REFERENCES volumes(id) ON DELETE CASCADE,
    project_id UUID REFERENCES projects(id) ON DELETE CASCADE,
    size_gb INTEGER NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'creating',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Glance tables
CREATE TABLE IF NOT EXISTS images (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    project_id UUID REFERENCES projects(id) ON DELETE SET NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'queued',
    visibility VARCHAR(50) DEFAULT 'private', -- 'public', 'private', 'shared', 'community'
    size_bytes BIGINT,
    disk_format VARCHAR(50), -- 'qcow2', 'raw', 'iso', etc.
    container_format VARCHAR(50), -- 'bare', 'ovf', etc.
    min_disk_gb INTEGER DEFAULT 0,
    min_ram_mb INTEGER DEFAULT 0,
    checksum VARCHAR(255),
    rbd_pool VARCHAR(255),
    rbd_image VARCHAR(255),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for performance
CREATE INDEX idx_instances_project_id ON instances(project_id);
CREATE INDEX idx_instances_status ON instances(status);
CREATE INDEX idx_networks_project_id ON networks(project_id);
CREATE INDEX idx_ports_network_id ON ports(network_id);
CREATE INDEX idx_ports_device_id ON ports(device_id);
CREATE INDEX idx_volumes_project_id ON volumes(project_id);
CREATE INDEX idx_volumes_attached_to ON volumes(attached_to_instance_id);
CREATE INDEX idx_images_project_id ON images(project_id);
CREATE INDEX idx_images_visibility ON images(visibility);
CREATE INDEX idx_role_assignments_user_project ON role_assignments(user_id, project_id);
