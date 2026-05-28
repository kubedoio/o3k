-- SCS-0114-v1 default volume types.
-- See: https://docs.scs.community/standards/scs-0114-v1-volume-type-standard/
--
-- The standard advertises encryption and replication via tags in the volume
-- type description ([scs:encrypted], [scs:replicated]). We mirror those into
-- queryable extra-specs (scs:encrypted, scs:replicated, scs:availability-zone)
-- so SCS-aware clients can filter on them through the standard volume-type
-- extra-specs API.
--
-- Three reference types covering the canonical combinations:
--   default          — unencrypted, single-AZ (the baseline)
--   encrypted        — at-rest encryption
--   replicated       — multi-AZ replication
--
-- Operators replace these with backend-specific types as the deployment grows.

INSERT INTO volume_types (id, name, description, is_public, extra_specs) VALUES
    ('00000000-0000-0000-0000-0000000006c1', 'scs-default',
     'SCS default volume type — unencrypted, single-AZ', true,
     '{"scs:encrypted": "false", "scs:replicated": "false", "scs:availability-zone": "nova"}'::jsonb),
    ('00000000-0000-0000-0000-0000000006c2', 'scs-encrypted',
     'SCS encrypted volume type [scs:encrypted]', true,
     '{"scs:encrypted": "true", "scs:replicated": "false", "scs:availability-zone": "nova"}'::jsonb),
    ('00000000-0000-0000-0000-0000000006c3', 'scs-replicated',
     'SCS replicated volume type [scs:replicated]', true,
     '{"scs:encrypted": "false", "scs:replicated": "true", "scs:availability-zone": "nova"}'::jsonb)
ON CONFLICT (name) DO NOTHING;
