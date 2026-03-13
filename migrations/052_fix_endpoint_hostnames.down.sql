-- Revert endpoint URLs back to localhost
UPDATE endpoints SET url = 'http://localhost:35357/v3' WHERE service_id = '00000000-0000-0000-0000-000000000010';
UPDATE endpoints SET url = 'http://localhost:8774/v2.1' WHERE service_id = '00000000-0000-0000-0000-000000000011';
UPDATE endpoints SET url = 'http://localhost:9696/v2.0' WHERE service_id = '00000000-0000-0000-0000-000000000012';
UPDATE endpoints SET url = 'http://localhost:8776/v3' WHERE service_id = '00000000-0000-0000-0000-000000000013';
UPDATE endpoints SET url = 'http://localhost:9292/v2' WHERE service_id = '00000000-0000-0000-0000-000000000014';
