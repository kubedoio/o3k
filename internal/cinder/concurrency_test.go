package cinder

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/cobaltcore-dev/o3k/internal/database"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// setupCinderTestDB creates a SQLite adapter in a temp directory and builds
// the minimal schema needed for cinder volume tests (no FK constraints so we
// don't need the full migration chain).
func setupCinderTestDB(t *testing.T) *database.SQLiteAdapter {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "cinder_test.db")
	adapter, err := database.NewSQLiteAdapter(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteAdapter: %v", err)
	}
	t.Cleanup(adapter.Close)

	ctx := t.Context()

	// Minimal volumes table — no FK constraints for test isolation.
	_, err = adapter.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS volumes (
			id TEXT PRIMARY KEY,
			name TEXT,
			project_id TEXT NOT NULL,
			user_id TEXT,
			size_gb INTEGER NOT NULL DEFAULT 1,
			status TEXT NOT NULL DEFAULT 'available',
			bootable INTEGER DEFAULT 0,
			attached_to_instance_id TEXT,
			rbd_pool TEXT,
			rbd_image TEXT,
			availability_zone TEXT DEFAULT 'nova',
			encrypted INTEGER DEFAULT 0,
			volume_type TEXT DEFAULT '__DEFAULT__',
			created_at TEXT DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT DEFAULT CURRENT_TIMESTAMP
		)`)
	if err != nil {
		t.Fatalf("create volumes table: %v", err)
	}

	// instances table for attach instance-exists check.
	_, err = adapter.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS instances (
			id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL
		)`)
	if err != nil {
		t.Fatalf("create instances table: %v", err)
	}

	// cinder_quotas table — quota queries fall through to defaults on ErrNoRows.
	_, err = adapter.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS cinder_quotas (
			project_id TEXT NOT NULL,
			resource TEXT NOT NULL,
			"limit" INTEGER NOT NULL,
			PRIMARY KEY(project_id, resource)
		)`)
	if err != nil {
		t.Fatalf("create cinder_quotas table: %v", err)
	}

	return adapter
}

// insertVolume inserts a test volume and returns its ID.
func insertVolume(t *testing.T, db database.DBIF, projectID, status string, sizeGB int) string {
	t.Helper()
	id := uuid.New().String()
	_, err := db.Exec(t.Context(), `
		INSERT INTO volumes (id, project_id, size_gb, status, name, rbd_pool, rbd_image)
		VALUES ($1, $2, $3, $4, 'test-vol', '', '')`,
		id, projectID, sizeGB, status)
	if err != nil {
		t.Fatalf("insertVolume: %v", err)
	}
	return id
}

// insertInstance inserts a test instance and returns its ID.
func insertInstance(t *testing.T, db database.DBIF, projectID string) string {
	t.Helper()
	id := uuid.New().String()
	_, err := db.Exec(t.Context(), `INSERT INTO instances (id, project_id) VALUES ($1, $2)`, id, projectID)
	if err != nil {
		t.Fatalf("insertInstance: %v", err)
	}
	return id
}

// newTestRouter builds a minimal gin router that injects projectID and userID
// into the context before invoking the handler, so that no real JWT middleware
// is required. The router properly flushes response headers.
func newTestRouter(svc *Service, projectID string) *gin.Engine {
	r := gin.New()
	// Auth injection middleware.
	r.Use(func(c *gin.Context) {
		c.Set("project_id", projectID)
		c.Set("user_id", "test-user")
		c.Next()
	})
	r.POST("/volumes/:id/action", svc.VolumeAction)
	r.POST("/volumes", svc.CreateVolume)
	return r
}

// doVolumeAction sends a POST /volumes/:id/action request through a real gin router.
func doVolumeAction(svc *Service, projectID, volumeID string, body map[string]any) *httptest.ResponseRecorder {
	payload, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, "/volumes/"+volumeID+"/action", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	newTestRouter(svc, projectID).ServeHTTP(w, req)
	return w
}

// doCreateVolume sends a POST /volumes request through a real gin router.
func doCreateVolume(svc *Service, projectID string, sizeGB int) *httptest.ResponseRecorder {
	body := map[string]any{
		"volume": map[string]any{
			"size": sizeGB,
			"name": "test-vol-" + fmt.Sprint(time.Now().UnixNano()),
		},
	}
	payload, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, "/volumes", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	newTestRouter(svc, projectID).ServeHTTP(w, req)
	return w
}

// ---------------------------------------------------------------------------
// Test 1: TestConcurrentAttach
//
// 10 goroutines all try to attach the same volume. Only one should succeed
// (202 Accepted); the rest should get 409 Conflict because the status
// transitions from "available" to "in-use" atomically inside a transaction.
// ---------------------------------------------------------------------------
func TestConcurrentAttach(t *testing.T) {
	db := setupCinderTestDB(t)

	projectID := "proj-" + uuid.New().String()
	volumeID := insertVolume(t, db, projectID, "available", 10)
	instanceID := insertInstance(t, db, projectID)

	svc := NewServiceWithDB(db, "stub", "testpool", "")

	const goroutines = 10
	results := make([]int, goroutines)
	var wg sync.WaitGroup
	for i := range goroutines {
		wg.Go(func() {
			w := doVolumeAction(svc, projectID, volumeID, map[string]any{
				"os-attach": map[string]any{
					"instance_uuid": instanceID,
				},
			})
			results[i] = w.Code
		})
	}
	wg.Wait()

	successes := 0
	for _, code := range results {
		if code == http.StatusAccepted {
			successes++
		}
	}
	if successes != 1 {
		t.Errorf("expected exactly 1 successful attach, got %d (results: %v)", successes, results)
	}

	// Volume must end up in-use.
	var finalStatus string
	if err := db.QueryRow(t.Context(),
		"SELECT status FROM volumes WHERE id = $1", volumeID,
	).Scan(&finalStatus); err != nil {
		t.Fatalf("query final status: %v", err)
	}
	if finalStatus != "in-use" {
		t.Errorf("expected final status 'in-use', got %q", finalStatus)
	}
}

// ---------------------------------------------------------------------------
// Test 2: TestConcurrentExtend
//
// 5 goroutines all try to extend the same volume from 10 GB to 20 GB.
// Only one should succeed; the rest see status != "available" and get 409.
// The final size must be exactly 20 GB, not 10 GB (lost update) or any other value.
// ---------------------------------------------------------------------------
func TestConcurrentExtend(t *testing.T) {
	db := setupCinderTestDB(t)

	projectID := "proj-" + uuid.New().String()
	volumeID := insertVolume(t, db, projectID, "available", 10)

	svc := NewServiceWithDB(db, "stub", "testpool", "")

	const goroutines = 5
	var successes atomic.Int32
	var wg sync.WaitGroup
	for range goroutines {
		wg.Go(func() {
			w := doVolumeAction(svc, projectID, volumeID, map[string]any{
				"os-extend": map[string]any{
					"new_size": 20,
				},
			})
			if w.Code == http.StatusAccepted {
				successes.Add(1)
			}
		})
	}
	wg.Wait()

	if successes.Load() != 1 {
		t.Errorf("expected exactly 1 successful extend, got %d", successes.Load())
	}

	// Verify final size is 20 (the extended value).
	var finalSize int
	if err := db.QueryRow(t.Context(),
		"SELECT size_gb FROM volumes WHERE id = $1", volumeID,
	).Scan(&finalSize); err != nil {
		t.Fatalf("query final size: %v", err)
	}
	if finalSize != 20 {
		t.Errorf("expected final size 20 GB, got %d GB", finalSize)
	}
}

// ---------------------------------------------------------------------------
// Test 3: TestQuotaEnforcementConcurrent
//
// Fill the project to (limit - 1) volumes, then spawn 5 goroutines each
// trying to create one more. With the default limit of 10 volumes and 9
// already present, at most 1 new volume should be created.
// ---------------------------------------------------------------------------
func TestQuotaEnforcementConcurrent(t *testing.T) {
	db := setupCinderTestDB(t)

	const defaultMaxVolumes = 10
	projectID := "proj-" + uuid.New().String()

	// Pre-fill to limit-1.
	for range defaultMaxVolumes - 1 {
		insertVolume(t, db, projectID, "available", 1)
	}

	svc := NewServiceWithDB(db, "stub", "testpool", "")

	const goroutines = 5
	var successes atomic.Int32
	var wg sync.WaitGroup
	for range goroutines {
		wg.Go(func() {
			w := doCreateVolume(svc, projectID, 1)
			// CreateVolume returns 200 on success (gin default for c.JSON)
			// or 201 in some implementations; treat any 2xx as success.
			if w.Code >= 200 && w.Code < 300 {
				successes.Add(1)
			}
		})
	}
	wg.Wait()

	if successes.Load() > 1 {
		t.Errorf("quota should allow at most 1 additional volume, got %d successes", successes.Load())
	}

	// Confirm total volume count does not exceed the limit.
	var totalCount int
	if err := db.QueryRow(t.Context(),
		"SELECT COUNT(*) FROM volumes WHERE project_id = $1 AND status != 'deleted'", projectID,
	).Scan(&totalCount); err != nil {
		t.Fatalf("count volumes: %v", err)
	}
	if totalCount > defaultMaxVolumes {
		t.Errorf("total volumes %d exceeds quota limit %d", totalCount, defaultMaxVolumes)
	}
}
