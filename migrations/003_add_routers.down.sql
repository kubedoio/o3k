-- Drop L3 router tables

DROP INDEX IF EXISTS idx_router_routes_router_id;
DROP INDEX IF EXISTS idx_floating_ips_network_id;
DROP INDEX IF EXISTS idx_floating_ips_router_id;
DROP INDEX IF EXISTS idx_floating_ips_port_id;
DROP INDEX IF EXISTS idx_floating_ips_project_id;
DROP INDEX IF EXISTS idx_router_interfaces_port_id;
DROP INDEX IF EXISTS idx_router_interfaces_router_id;
DROP INDEX IF EXISTS idx_routers_project_id;

DROP TABLE IF EXISTS router_routes;
DROP TABLE IF EXISTS floating_ips;
DROP TABLE IF EXISTS router_interfaces;
DROP TABLE IF EXISTS routers;
