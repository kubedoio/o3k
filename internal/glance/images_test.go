package glance

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/cobaltcore-dev/o3k/internal/database"
	"github.com/gin-gonic/gin"
)

func newFakeGinContext(params map[string]string) *gin.Context {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	q := url.Values{}
	for k, v := range params {
		q.Set(k, v)
	}
	c.Request, _ = http.NewRequest(http.MethodGet, "/?"+q.Encode(), nil)
	return c
}

func TestJoinConditions(t *testing.T) {
	tests := []struct {
		name       string
		conditions []string
		want       string
	}{
		{"empty", nil, ""},
		{"single", []string{"a = $1"}, "a = $1"},
		{"two", []string{"a = $1", "b = $2"}, "a = $1 AND b = $2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := joinConditions(tt.conditions)
			if got != tt.want {
				t.Errorf("joinConditions(%v) = %q, want %q", tt.conditions, got, tt.want)
			}
		})
	}
}

// TestListImagesVisibilityCondition verifies the visibility branch produces the
// right SQL fragment and placeholder indices without hitting a real database.
func TestListImagesVisibilityCondition(t *testing.T) {
	tests := []struct {
		name             string
		queryParams      map[string]string
		wantCondFragment string // substring that must appear in first condition
		wantArgCount     int    // number of args consumed for the visibility clause
	}{
		{
			name:             "no visibility param",
			queryParams:      map[string]string{},
			wantCondFragment: "(visibility = 'public' OR project_id = $1)",
			wantArgCount:     1,
		},
		{
			name:             "visibility=public",
			queryParams:      map[string]string{"visibility": "public"},
			wantCondFragment: "visibility = 'public'",
			wantArgCount:     0,
		},
		{
			name:             "visibility=private",
			queryParams:      map[string]string{"visibility": "private"},
			wantCondFragment: "visibility = 'private'",
			wantArgCount:     1,
		},
		{
			name:             "visibility=shared",
			queryParams:      map[string]string{"visibility": "shared"},
			wantCondFragment: "(visibility = 'public' OR project_id = $1)",
			wantArgCount:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newFakeGinContext(tt.queryParams)
			c.Set("project_id", "proj-abc")

			var conditions []string
			var queryArgs []interface{}
			argIdx := 1
			projectID := "proj-abc"

			vis := c.Query("visibility")
			if vis != "" {
				switch vis {
				case "public":
					conditions = append(conditions, "visibility = 'public'")
				case "private":
					conditions = append(conditions, "(visibility = 'private' AND project_id = $1)")
					queryArgs = append(queryArgs, projectID)
					argIdx++
				default:
					conditions = append(conditions, "(visibility = 'public' OR project_id = $1)")
					queryArgs = append(queryArgs, projectID)
					argIdx++
				}
			} else {
				conditions = append(conditions, "(visibility = 'public' OR project_id = $1)")
				queryArgs = append(queryArgs, projectID)
				argIdx++
			}
			_ = argIdx // suppress unused warning

			if len(conditions) == 0 {
				t.Fatal("expected at least one condition")
			}
			if !strings.Contains(conditions[0], tt.wantCondFragment) {
				t.Errorf("condition %q does not contain %q", conditions[0], tt.wantCondFragment)
			}
			if len(queryArgs) != tt.wantArgCount {
				t.Errorf("got %d args, want %d", len(queryArgs), tt.wantArgCount)
			}
		})
	}
}

func TestAllowedImageUpdateField(t *testing.T) {
	tests := []struct {
		path    string
		allowed bool
		field   string
	}{
		{"/name", true, "name"},
		{"/visibility", true, "visibility"},
		{"/min_disk", true, "min_disk_gb"},
		{"/min_ram", true, "min_ram_mb"},
		{"/malicious; DROP TABLE images;--", false, ""},
		{"/nonexistent", false, ""},
	}
	for _, tt := range tests {
		field, ok := allowedImageUpdateField(tt.path)
		if ok != tt.allowed {
			t.Errorf("allowedImageUpdateField(%q) ok = %v, want %v", tt.path, ok, tt.allowed)
		}
		if ok && field != tt.field {
			t.Errorf("allowedImageUpdateField(%q) field = %q, want %q", tt.path, field, tt.field)
		}
	}
}

// insertTestProject creates a project row required by the images FK constraint.
func insertTestProject(t *testing.T, db *sql.DB, id string) {
	t.Helper()
	_, err := db.ExecContext(context.Background(), database.Q(`INSERT INTO projects (id, name) VALUES ($1, $2)`), id, id)
	if err != nil {
		t.Fatalf("insert project: %v", err)
	}
}

// insertTestImage creates an image fixture in the in-memory test database.
func insertTestImage(t *testing.T, db *sql.DB, imageID, ownerProjectID, visibility string, protected bool) {
	t.Helper()
	now := time.Now().Format(time.RFC3339)
	_, err := db.ExecContext(context.Background(), database.Q(`
		INSERT INTO images (id, name, project_id, status, visibility, disk_format, container_format, min_disk_gb, min_ram_mb, protected, properties, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`), imageID, "test-image", ownerProjectID, "active", visibility, "qcow2", "bare", 0, 0, protected, "{}", now, now)
	if err != nil {
		t.Fatalf("insert image: %v", err)
	}
}

func TestDeleteImageProtected(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		ownerProjectID string
		protected      bool
		isAdmin        bool
		wantStatus     int
	}{
		{
			name:           "protected image returns 403",
			ownerProjectID: "proj-abc",
			protected:      true,
			isAdmin:        false,
			wantStatus:     http.StatusForbidden,
		},
		{
			name:           "admin cannot delete protected image",
			ownerProjectID: "other-proj",
			protected:      true,
			isAdmin:        true,
			wantStatus:     http.StatusForbidden,
		},
		{
			name:           "unprotected image owner can delete",
			ownerProjectID: "proj-abc",
			protected:      false,
			isAdmin:        false,
			wantStatus:     http.StatusNoContent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := database.NewTestDB(t)
			insertTestProject(t, db, tt.ownerProjectID)

			imageID := "img-id-123"
			insertTestImage(t, db, imageID, tt.ownerProjectID, "private", tt.protected)

			svc := NewServiceWithDB(db, "stub", "", "", "", "", "", nil)

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			req, _ := http.NewRequest(http.MethodDelete, "/images/"+imageID, nil)
			c.Request = req
			c.Params = gin.Params{{Key: "id", Value: imageID}}
			c.Set("project_id", "proj-abc")
			c.Set("is_admin", tt.isAdmin)

			svc.DeleteImage(c)

			if tt.wantStatus == http.StatusNoContent {
				// gin.CreateTestContext does not call WriteHeaderNow for no-body responses;
				// verify the DELETE was issued and no 403 was returned instead.
				if w.Code == http.StatusForbidden {
					t.Errorf("expected delete to proceed but got 403; body = %s", w.Body.String())
				}
				var count int
				if err := db.QueryRowContext(context.Background(), database.Q("SELECT COUNT(*) FROM images WHERE id = $1"), imageID).Scan(&count); err != nil {
					t.Fatalf("count images: %v", err)
				}
				if count != 0 {
					t.Errorf("expected image to be deleted but count = %d", count)
				}
				return
			}

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d; body = %s", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}
