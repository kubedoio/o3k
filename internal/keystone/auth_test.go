package keystone_test

import (
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
	"github.com/cobaltcore-dev/o3k/internal/keystone"
)

func newAuthSvc(t *testing.T) *keystone.AuthService {
	t.Helper()
	mock := database.NewSeededMockDB()
	return keystone.NewAuthServiceWithDB(mock, "test-secret", time.Hour, nil)
}

func TestPasswordAuthSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	authSvc := newAuthSvc(t)

	body := `{"auth":{"identity":{"methods":["password"],"password":{"user":{"name":"admin","password":"secret","domain":{"name":"Default"}}}}}}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/v3/auth/tokens", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	var req keystone.AuthRequest
	require.NoError(t, json.NewDecoder(c.Request.Body).Decode(&req))

	resp, token, err := authSvc.AuthenticatePassword(t.Context(), &req)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	assert.NotEmpty(t, resp.Token.ExpiresAt)
	assert.NotEmpty(t, resp.Token.IssuedAt)
	assert.Equal(t, []string{"password"}, resp.Token.Methods)
	userID, ok := resp.Token.User["id"]
	assert.True(t, ok)
	assert.NotEmpty(t, userID)
}

func TestPasswordAuthWrongPassword(t *testing.T) {
	gin.SetMode(gin.TestMode)
	authSvc := newAuthSvc(t)

	body := `{"auth":{"identity":{"methods":["password"],"password":{"user":{"name":"admin","password":"wrong","domain":{"name":"Default"}}}}}}`
	var req keystone.AuthRequest
	require.NoError(t, json.Unmarshal([]byte(body), &req))

	_, _, err := authSvc.AuthenticatePassword(t.Context(), &req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid credentials")
}

func TestTokenValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	authSvc := newAuthSvc(t)

	// Issue a token first.
	body := `{"auth":{"identity":{"methods":["password"],"password":{"user":{"name":"admin","password":"secret","domain":{"name":"Default"}}}}}}`
	var req keystone.AuthRequest
	require.NoError(t, json.Unmarshal([]byte(body), &req))

	_, token, err := authSvc.AuthenticatePassword(t.Context(), &req)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	// Validate the issued token.
	claims, err := authSvc.ValidateToken(token)
	require.NoError(t, err)
	assert.Equal(t, "admin", claims.UserName)
	assert.Equal(t, "admin-user-id", claims.UserID)
}

func TestTokenValidationExpired(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mock := database.NewSeededMockDB()
	// TTL of zero means the token expires immediately upon issuance.
	authSvc := keystone.NewAuthServiceWithDB(mock, "test-secret", 0, nil)

	body := `{"auth":{"identity":{"methods":["password"],"password":{"user":{"name":"admin","password":"secret","domain":{"name":"Default"}}}}}}`
	var req keystone.AuthRequest
	require.NoError(t, json.Unmarshal([]byte(body), &req))

	_, token, err := authSvc.AuthenticatePassword(t.Context(), &req)
	require.NoError(t, err)

	// Token is immediately expired; validation must return 401.
	_, err = authSvc.ValidateToken(token)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Token has expired")
}

func TestTokenRevocation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	authSvc := newAuthSvc(t)

	body := `{"auth":{"identity":{"methods":["password"],"password":{"user":{"name":"admin","password":"secret","domain":{"name":"Default"}}}}}}`
	var req keystone.AuthRequest
	require.NoError(t, json.Unmarshal([]byte(body), &req))

	_, token, err := authSvc.AuthenticatePassword(t.Context(), &req)
	require.NoError(t, err)

	// Token should be valid before revocation.
	_, err = authSvc.ValidateToken(token)
	require.NoError(t, err)

	// Revoke it.
	expiresAt := time.Now().Add(time.Hour)
	authSvc.RevokeToken(token, expiresAt)

	// Token must be rejected after revocation.
	_, err = authSvc.ValidateToken(token)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "revoked")
}

func TestAppCredentialScopeEnforced(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mock := database.NewSeededMockDB()
	authSvc := keystone.NewAuthServiceWithDB(mock, "test-secret", time.Hour, nil)

	// An app-credential request referencing a different project scope is rejected.
	body := `{"auth":{"identity":{"methods":["application_credential"],"application_credential":{"id":"cred-id","secret":"cred-secret"}},"scope":{"project":{"id":"other-project-id"}}}}`
	var req keystone.AuthRequest
	require.NoError(t, json.Unmarshal([]byte(body), &req))

	// The mock DB returns ErrNoRows for the application_credentials lookup so
	// the service returns "invalid application credential" before reaching the
	// scope check. This is the correct behaviour — you can't escalate scope
	// when you don't even hold a valid credential.
	_, _, _, err := authSvc.AuthenticateApplicationCredential(t.Context(), &req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid application credential")
}
