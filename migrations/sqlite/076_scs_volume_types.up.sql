-- SCS-0114-v1 default volume types (sqlite mirror).
-- See: https://docs.scs.community/standards/scs-0114-v1-volume-type-standard/
--
-- The standard advertises encryption and replication via tags in the volume
-- type description ([scs:encrypted], [scs:replicated]). We mirror those into
-- queryable extra-specs (scs:encrypted, scs:replicated, scs:availability-zone)
-- so SCS-aware clients can filter on them through the standard volume-type
-- extra-specs API.
--
-- sqlite stores extra_specs as TEXT, so the JSON document is a string.

INSERT OR IGNORE INTO volume_types (id, name, description, is_public, extra_specs) VALUES
    ('00000000-0000-0000-0000-0000000006c1', 'scs-default',
     'SCS default volume type — unencrypted, single-AZ', 1,
     '{"scs:encrypted":"false","scs:replicated":"false","scs:availability-zone":"nova"}'),
    ('00000000-0000-0000-0000-0000000006c2', 'scs-encrypted',
     'SCS encrypted volume type [scs:encrypted]', 1,
     '{"scs:encrypted":"true","scs:replicated":"false","scs:availability-zone":"nova"}'),
    ('00000000-0000-0000-0000-0000000006c3', 'scs-replicated',
     'SCS replicated volume type [scs:replicated]', 1,
     '{"scs:encrypted":"false","scs:replicated":"true","scs:availability-zone":"nova"}');
