-- Revert Neutron endpoint URL to include /v2.0
UPDATE endpoints
SET url = REPLACE(url, ':9696', ':9696/v2.0')
WHERE service_id = '00000000-0000-0000-0000-000000000012' -- Neutron service
  AND url LIKE '%:9696'
  AND url NOT LIKE '%:9696/v2.0%';
