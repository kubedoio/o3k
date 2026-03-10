package keystone_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack/identity/v3/projects"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestKeystoneCreateProject_Contract tests POST /v3/projects endpoint
func TestKeystoneCreateProject_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupClient(t)

	// Generate unique project name
	projectName := "test-project-" + uuid.New().String()[:8]

	// Test: Create project using gophercloud SDK
	createOpts := projects.CreateOpts{
		Name:        projectName,
		Description: "Test project for contract testing",
		DomainID:    "00000000-0000-0000-0000-000000000100", // Default domain
		Enabled:     gophercloud.Enabled,
	}

	project, err := projects.Create(client, createOpts).Extract()

	// Assertions: Verify OpenStack API contract
	require.NoError(t, err, "CreateProject should succeed")
	assert.NotEmpty(t, project.ID, "Project ID should be set")
	assert.Equal(t, projectName, project.Name, "Project name should match")
	assert.True(t, project.Enabled, "Project should be enabled by default")
	assert.Equal(t, "Test project for contract testing", project.Description)
	assert.Equal(t, "00000000-0000-0000-0000-000000000100", project.DomainID)

	// Cleanup
	defer func() {
		err := projects.Delete(client, project.ID).ExtractErr()
		if err != nil {
			t.Logf("Cleanup failed: %v", err)
		}
	}()
}

// TestKeystoneUpdateProject_Contract tests PATCH /v3/projects/:id endpoint
func TestKeystoneUpdateProject_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupClient(t)

	// Setup: Create project first
	projectName := "test-project-update-" + uuid.New().String()[:8]
	createdProject, err := projects.Create(client, projects.CreateOpts{
		Name:        projectName,
		Description: "Initial description",
		DomainID:    "00000000-0000-0000-0000-000000000100",
		Enabled:     gophercloud.Enabled,
	}).Extract()
	require.NoError(t, err, "Setup: CreateProject should succeed")
	defer projects.Delete(client, createdProject.ID)

	// Test: Update project description
	newDescription := "Updated project description"
	updateOpts := projects.UpdateOpts{
		Description: &newDescription,
	}

	updatedProject, err := projects.Update(client, createdProject.ID, updateOpts).Extract()

	// Assertions
	require.NoError(t, err, "UpdateProject should succeed")
	assert.Equal(t, createdProject.ID, updatedProject.ID, "Project ID should remain the same")
	assert.Equal(t, "Updated project description", updatedProject.Description, "Description should be updated")
	assert.Equal(t, projectName, updatedProject.Name, "Name should remain unchanged")
}

// TestKeystoneUpdateProjectEnabled_Contract tests enabling/disabling projects
func TestKeystoneUpdateProjectEnabled_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupClient(t)

	// Setup: Create project
	projectName := "test-project-enabled-" + uuid.New().String()[:8]
	createdProject, err := projects.Create(client, projects.CreateOpts{
		Name:     projectName,
		DomainID: "00000000-0000-0000-0000-000000000100",
		Enabled:  gophercloud.Enabled,
	}).Extract()
	require.NoError(t, err)
	defer projects.Delete(client, createdProject.ID)

	// Test: Disable project
	disabled := false
	updatedProject, err := projects.Update(client, createdProject.ID, projects.UpdateOpts{
		Enabled: &disabled,
	}).Extract()

	// Assertions
	require.NoError(t, err)
	assert.False(t, updatedProject.Enabled, "Project should be disabled")

	// Test: Re-enable project
	enabled := true
	updatedProject, err = projects.Update(client, createdProject.ID, projects.UpdateOpts{
		Enabled: &enabled,
	}).Extract()

	require.NoError(t, err)
	assert.True(t, updatedProject.Enabled, "Project should be re-enabled")
}

// TestKeystoneDeleteProject_Contract tests DELETE /v3/projects/:id endpoint
func TestKeystoneDeleteProject_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupClient(t)

	// Setup: Create project
	projectName := "test-project-delete-" + uuid.New().String()[:8]
	createdProject, err := projects.Create(client, projects.CreateOpts{
		Name:     projectName,
		DomainID: "00000000-0000-0000-0000-000000000100",
	}).Extract()
	require.NoError(t, err)

	// Test: Delete project
	err = projects.Delete(client, createdProject.ID).ExtractErr()

	// Assertions
	require.NoError(t, err, "DeleteProject should succeed")

	// Verify deletion: GET should return 404
	_, err = projects.Get(client, createdProject.ID).Extract()
	assert.Error(t, err, "GET after DELETE should fail with 404")
	assert.Contains(t, err.Error(), "404", "Should be 404 Not Found")
}

// TestKeystoneDeleteNonExistentProject_Contract tests 404 handling
func TestKeystoneDeleteNonExistentProject_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupClient(t)

	// Test: Delete non-existent project
	fakeProjectID := uuid.New().String()
	err := projects.Delete(client, fakeProjectID).ExtractErr()

	// Assertions: Should return 404
	require.Error(t, err, "Delete non-existent project should fail")
	assert.Contains(t, err.Error(), "404", "Should be 404 Not Found")
}
