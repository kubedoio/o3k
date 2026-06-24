//go:build vxlan_integration
// +build vxlan_integration

package neutron

import (
	"database/sql"
)

// setDB injects a *sql.DB for tests. Mirrors NodeRegistry's `db` field
// pattern: production callers use the global database.DB; tests need
// two coordinator instances sharing one pool without touching the
// global. Lives in a build-tagged file so it never ships in production
// binaries.
func (vc *VXLANCoordinator) setDB(db *sql.DB) {
	vc.db = db
}
