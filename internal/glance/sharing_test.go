package glance

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cobaltcore-dev/o3k/internal/database"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// insertImageMember creates an image_members row for sharing tests.
func insertImageMember(t *testing.T, db *sql.DB, imageID, memberID, status string) {
	t.Helper()
	now := time.Now().Format(time.RFC3339)
	_, err := db.ExecContext(context.Background(), database.Q(`
		INSERT INTO image_members (id, image_id, member_id, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`), "member-"+memberID, imageID, memberID, status, now, now)
	if err != nil {
		t.Fatalf("insert image member: %v", err)
	}
}

// glanceGetImageMock issues a GET /images/:id request using a real in-memory
// SQLite DB backend and returns the HTTP status code and decoded response.
func glanceGetImageMock(t *testing.T, db *sql.DB, imageID, projectID string) (int, map[string]interface{}) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	svc := NewServiceWithDB(db, "stub", "", "", "", "", "", nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/images/"+imageID, nil)
	c.Params = gin.Params{{Key: "id", Value: imageID}}
	c.Set("project_id", projectID)
	c.Set("user_id", "some-user")

	svc.GetImage(c)

	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	return w.Code, body
}

// TestGetImageAsOwner verifies that the image owner can access their own image.
func TestGetImageAsOwner(t *testing.T) {
	db := database.NewTestDB(t)
	insertTestImage(t, db, "img-001", "owner-project", "private", false)
	code, body := glanceGetImageMock(t, db, "img-001", "owner-project")

	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "img-001", body["id"])
}

// TestGetImageAsAcceptedMember verifies that an accepted member can access a
// shared private image.
func TestGetImageAsAcceptedMember(t *testing.T) {
	db := database.NewTestDB(t)
	insertTestImage(t, db, "img-002", "owner-project", "private", false)
	insertImageMember(t, db, "img-002", "member-project", "accepted")
	code, _ := glanceGetImageMock(t, db, "img-002", "member-project")
	assert.Equal(t, http.StatusOK, code, "accepted member must be able to access image")
}

// TestGetImageAsPendingMemberFails verifies that a pending (non-accepted)
// member gets 404.
func TestGetImageAsPendingMemberFails(t *testing.T) {
	db := database.NewTestDB(t)
	insertTestImage(t, db, "img-003", "owner-project", "private", false)
	insertImageMember(t, db, "img-003", "pending-project", "pending")
	code, _ := glanceGetImageMock(t, db, "img-003", "pending-project")
	assert.Equal(t, http.StatusNotFound, code, "pending member must not be able to access image")
}

// TestGetImageAsNonMemberFails verifies that a project with no relationship
// to the image gets 404 on a private image.
func TestGetImageAsNonMemberFails(t *testing.T) {
	db := database.NewTestDB(t)
	insertTestImage(t, db, "img-004", "owner-project", "private", false)
	code, _ := glanceGetImageMock(t, db, "img-004", "stranger-project")
	assert.Equal(t, http.StatusNotFound, code, "non-member must not be able to access private image")
}

// TestPublicImageAccessibleToAll verifies that any project (including empty)
// can access a public image.
func TestPublicImageAccessibleToAll(t *testing.T) {
	tests := []struct {
		name      string
		projectID string
	}{
		{"owner access", "owner-project"},
		{"random project", "random-project"},
		{"empty project", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := database.NewTestDB(t)
			insertTestImage(t, db, "img-005", "owner-project", "public", false)
			code, body := glanceGetImageMock(t, db, "img-005", tt.projectID)
			require.Equal(t, http.StatusOK, code,
				"public image must be accessible; projectID=%q body=%v", tt.projectID, body)
			assert.Equal(t, "img-005", body["id"])
		})
	}
}
