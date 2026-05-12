package glance

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cobaltcore-dev/o3k/internal/database"
)

// emptyRows is an always-empty Rows implementation for test doubles.
type emptyRows struct{}

func (e *emptyRows) Next() bool             { return false }
func (e *emptyRows) Scan(dest ...any) error { return nil }
func (e *emptyRows) Close()                {}
func (e *emptyRows) Err() error            { return nil }

// getImageDB is a test double for GetImage. It controls what the images query
// returns (found/not-found) and what the image_tags query returns (empty).
type getImageDB struct {
	*database.MockDB
	// Non-nil row is returned when the images SELECT matches.
	imageRow *imageTestRow
}

// imageTestRow holds the column values that GetImage.Scan expects.
type imageTestRow struct {
	id              string
	name            string
	status          string
	visibility      string
	sizeBytes       sql.NullInt64
	diskFormat      string
	containerFormat string
	minDisk         int
	minRAM          int
	protected       bool
	checksum        sql.NullString
	osHashAlgo      sql.NullString
	osHashValue     sql.NullString
	createdAt       time.Time
	updatedAt       time.Time
	ownerProjectID  string
}

func (d *getImageDB) QueryRow(_ context.Context, sql string, args ...any) database.Row {
	if strings.Contains(sql, "FROM images") {
		if d.imageRow == nil {
			return &scanRow{err: database.ErrNoRows}
		}
		r := d.imageRow
		return &scanRow{values: []any{
			r.id, r.name, r.status, r.visibility,
			r.sizeBytes, r.diskFormat, r.containerFormat,
			r.minDisk, r.minRAM, r.protected,
			r.checksum, r.osHashAlgo, r.osHashValue,
			r.createdAt, r.updatedAt, r.ownerProjectID,
		}}
	}
	return &scanRow{err: database.ErrNoRows}
}

// Query overrides MockDB to return empty rows for tags queries.
func (d *getImageDB) Query(_ context.Context, sql string, args ...any) (database.Rows, error) {
	return &emptyRows{}, nil
}

func (d *getImageDB) Exec(ctx context.Context, s string, args ...any) (database.Result, error) {
	return d.MockDB.Exec(ctx, s, args...)
}
func (d *getImageDB) BeginTx(ctx context.Context, opts database.TxOptions) (database.Tx, error) {
	return d.MockDB.BeginTx(ctx, opts)
}

// Ensure interface compliance.
var _ database.DBIF = (*getImageDB)(nil)

// sampleImageRow returns a minimal valid imageTestRow for a given image id and visibility.
func sampleImageRow(id, ownerProjectID, visibility string) *imageTestRow {
	return &imageTestRow{
		id:              id,
		name:            "test-image",
		status:          "active",
		visibility:      visibility,
		diskFormat:      "qcow2",
		containerFormat: "bare",
		createdAt:       time.Now(),
		updatedAt:       time.Now(),
		ownerProjectID:  ownerProjectID,
	}
}

// glanceGetImageMock issues a GET /images/:id request using a mock DB backend
// and returns the HTTP status code and decoded response.
func glanceGetImageMock(t *testing.T, db database.DBIF, imageID, projectID string) (int, map[string]interface{}) {
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
	db := &getImageDB{
		MockDB:   database.NewMockDB(),
		imageRow: sampleImageRow("img-001", "owner-project", "private"),
	}
	code, body := glanceGetImageMock(t, db, "img-001", "owner-project")

	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "img-001", body["id"])
}

// TestGetImageAsAcceptedMember verifies that an accepted member can access a
// shared private image. The GetImage query performs the membership check
// inside the SQL WHERE clause; when the DB returns a row the test is green.
func TestGetImageAsAcceptedMember(t *testing.T) {
	// The getImageDB mock returns the image row unconditionally when the query
	// touches "FROM images" — this simulates the SQL WHERE clause resolving true
	// for an accepted member (the real DB does the member check in the query).
	db := &getImageDB{
		MockDB:   database.NewMockDB(),
		imageRow: sampleImageRow("img-002", "owner-project", "private"),
	}
	code, _ := glanceGetImageMock(t, db, "img-002", "member-project")
	assert.Equal(t, http.StatusOK, code, "accepted member must be able to access image")
}

// TestGetImageAsPendingMemberFails verifies that a pending (non-accepted)
// member gets 404. The mock returns ErrNoRows to simulate the SQL WHERE clause
// filtering out pending memberships.
func TestGetImageAsPendingMemberFails(t *testing.T) {
	db := &getImageDB{
		MockDB:   database.NewMockDB(),
		imageRow: nil, // no row → ErrNoRows
	}
	code, _ := glanceGetImageMock(t, db, "img-003", "pending-project")
	assert.Equal(t, http.StatusNotFound, code, "pending member must not be able to access image")
}

// TestGetImageAsNonMemberFails verifies that a project with no relationship
// to the image gets 404 on a private image.
func TestGetImageAsNonMemberFails(t *testing.T) {
	db := &getImageDB{
		MockDB:   database.NewMockDB(),
		imageRow: nil, // no row → ErrNoRows
	}
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
			db := &getImageDB{
				MockDB:   database.NewMockDB(),
				imageRow: sampleImageRow("img-005", "owner-project", "public"),
			}
			code, body := glanceGetImageMock(t, db, "img-005", tt.projectID)
			require.Equal(t, http.StatusOK, code,
				"public image must be accessible; projectID=%q body=%v", tt.projectID, body)
			assert.Equal(t, "img-005", body["id"])
		})
	}
}
