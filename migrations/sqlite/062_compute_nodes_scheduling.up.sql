ALTER TABLE compute_nodes ADD COLUMN total_vcpu INTEGER NOT NULL DEFAULT 0;
ALTER TABLE compute_nodes ADD COLUMN total_ram_mb INTEGER NOT NULL DEFAULT 0;
ALTER TABLE compute_nodes ADD COLUMN total_disk_gb INTEGER NOT NULL DEFAULT 0;
ALTER TABLE compute_nodes ADD COLUMN reserved_vcpu INTEGER NOT NULL DEFAULT 0;
ALTER TABLE compute_nodes ADD COLUMN reserved_ram_mb INTEGER NOT NULL DEFAULT 0;
ALTER TABLE compute_nodes ADD COLUMN reserved_disk_gb INTEGER NOT NULL DEFAULT 0;
ALTER TABLE compute_nodes ADD COLUMN stats_updated_at TEXT;
ALTER TABLE compute_nodes ADD COLUMN agent_stream_server_id TEXT;
