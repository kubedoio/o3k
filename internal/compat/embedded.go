package compat

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"github.com/cobaltcore-dev/o3k/internal/cinder"
	"github.com/cobaltcore-dev/o3k/internal/database"
	"github.com/cobaltcore-dev/o3k/internal/glance"
	"github.com/cobaltcore-dev/o3k/internal/keystone"
	"github.com/cobaltcore-dev/o3k/internal/middleware"
	"github.com/cobaltcore-dev/o3k/internal/neutron"
	"github.com/cobaltcore-dev/o3k/internal/nova"
	migrations "github.com/cobaltcore-dev/o3k/migrations"
)

// embeddedMux dispatches requests to the appropriate per-service router based
// on URL path prefix. This mirrors production where each service runs on its
// own port — here we keep them isolated to avoid Gin route conflicts.
type embeddedMux struct {
	routers []struct {
		prefix  string
		handler http.Handler
	}
}

func (m *embeddedMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	for _, sr := range m.routers {
		if strings.HasPrefix(r.URL.Path, sr.prefix) {
			sr.handler.ServeHTTP(w, r)
			return
		}
	}
	// Keystone is the catch-all (handles /v3 and /)
	m.routers[len(m.routers)-1].handler.ServeHTTP(w, r)
}

// seedEmbeddedDB inserts the standard admin user fixture used by the
// compatibility-check router so token issuance works out of the box.
func seedEmbeddedDB() {
	ctx := context.Background()
	hash, err := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	_, err = database.DB.ExecContext(ctx, database.Q(`
		INSERT INTO users (id, name, password_hash, enabled, domain_id)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (id) DO UPDATE SET password_hash = excluded.password_hash
	`), "admin-user-id", "admin", string(hash), true, "00000000-0000-0000-0000-000000000100")
	if err != nil {
		panic(err)
	}
	_, err = database.DB.ExecContext(ctx, database.Q(`
		INSERT INTO role_assignments (id, user_id, project_id, role_id)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (id) DO NOTHING
	`), "00000000-0000-0000-0000-000000000200", "admin-user-id", "00000000-0000-0000-0000-000000000002", "00000000-0000-0000-0000-000000000003")
	if err != nil {
		panic(err)
	}
}

// newServiceGin builds a minimal gin.Engine for a single service.
func newServiceGin() *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	return r
}

// NewEmbeddedRouter builds a per-service set of Gin routers with all five
// OpenStack services wired in stub mode against a MockDB. The multiplexer
// dispatches requests by URL prefix, mirroring the per-port production setup.
// The returned cleanup function restores the global database.DB.
func NewEmbeddedRouter() (http.Handler, func()) {
	origDB := database.DB
	if err := database.ConnectSQLite(context.Background(), ":memory:"); err != nil {
		panic(err)
	}
	if err := database.MigrateSQLiteFS(migrations.SQLiteFS); err != nil {
		panic(err)
	}
	seedEmbeddedDB()

	gin.SetMode(gin.ReleaseMode)

	authService := keystone.NewAuthService("compat-check-secret", 24*time.Hour, nil)

	// Keystone — handles /v3
	keystoneGin := newServiceGin()
	keystoneGin.Use(middleware.AuthMiddleware(authService))
	keystoneSvc := keystone.NewService(authService, nil)
	keystoneSvc.RegisterRoutes(keystoneGin.Group(""), middleware.RequireRole("admin"))

	// Nova — handles /v2.1, / (version discovery)
	novaGin := newServiceGin()
	novaGin.Use(middleware.AuthMiddleware(authService))
	novaSvc := nova.NewService("", "stub", nil)
	novaSvc.RegisterRoutes(novaGin.Group(""))

	// Neutron — handles /v2.0
	neutronGin := newServiceGin()
	neutronGin.Use(middleware.AuthMiddleware(authService))
	neutronSvc := neutron.NewService("stub", nil)
	neutronSvc.RegisterRoutes(neutronGin.Group(""))

	// Cinder — handles /v3 (storage), but dispatched via /v3/volumes prefix
	// In production Cinder runs on port 8776. We dispatch /v3/volumes,
	// /v3/snapshots, /v3/types, etc. to Cinder by mounting it under /cinder.
	// For the compat check we keep Cinder on its own engine and don't need
	// to reach it from the shared mux — the mux routes /v3 to Keystone,
	// and Cinder routes are exercised when compat tests call e.g. /v3/volumes.
	// To avoid the conflict we give Cinder a separate prefix-matched engine.
	cinderGin := newServiceGin()
	cinderGin.Use(middleware.AuthMiddleware(authService))
	cinderSvc := cinder.NewService("stub", "", "")
	cinderSvc.RegisterRoutes(cinderGin.Group(""))

	// Glance — handles /images, /schemas, /tasks
	glanceGin := newServiceGin()
	glanceGin.Use(middleware.AuthMiddleware(authService))
	glanceSvc := glance.NewService("stub", "", "", "", "", "", nil)
	glanceSvc.RegisterRoutes(glanceGin.Group(""))

	mux := &embeddedMux{
		routers: []struct {
			prefix  string
			handler http.Handler
		}{
			{prefix: "/v2.1", handler: novaGin},
			{prefix: "/v2.0", handler: neutronGin},
			{prefix: "/v3/volumes", handler: cinderGin},
			{prefix: "/v3/snapshots", handler: cinderGin},
			{prefix: "/v3/types", handler: cinderGin},
			{prefix: "/images", handler: glanceGin},
			{prefix: "/schemas", handler: glanceGin},
			{prefix: "/tasks", handler: glanceGin},
			// Keystone must be last — it catches /v3 and everything else.
			{prefix: "/", handler: keystoneGin},
		},
	}

	cleanup := func() {
		database.Close()
		database.DB = origDB
	}
	return mux, cleanup
}
