package policy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBasicRolePolicy(t *testing.T) {
	engine := NewEngine()
	engine.LoadPolicy(map[string]string{
		"compute:create": "role:member or role:admin",
	})

	assert.True(t, engine.Enforce("compute:create", nil, map[string]interface{}{
		"roles": []string{"member"},
	}))
	assert.True(t, engine.Enforce("compute:create", nil, map[string]interface{}{
		"roles": []string{"admin"},
	}))
	assert.False(t, engine.Enforce("compute:create", nil, map[string]interface{}{
		"roles": []string{"guest"},
	}))
	assert.False(t, engine.Enforce("compute:create", nil, map[string]interface{}{
		"roles": []string{},
	}))
}

func TestOwnershipPolicy(t *testing.T) {
	engine := NewEngine()
	engine.LoadPolicy(map[string]string{
		"owner":          "user_id:%(target.user_id)s",
		"admin_required": "role:admin",
		"compute:delete": "rule:admin_required or rule:owner",
	})

	assert.True(t, engine.Enforce("compute:delete",
		map[string]interface{}{"user_id": "user-123"},
		map[string]interface{}{"user_id": "user-123", "roles": []string{"member"}},
	))
	assert.False(t, engine.Enforce("compute:delete",
		map[string]interface{}{"user_id": "user-456"},
		map[string]interface{}{"user_id": "user-123", "roles": []string{"member"}},
	))
	assert.True(t, engine.Enforce("compute:delete",
		map[string]interface{}{"user_id": "user-456"},
		map[string]interface{}{"user_id": "admin-1", "roles": []string{"admin"}},
	))
}

func TestComplexAndOrRules(t *testing.T) {
	engine := NewEngine()
	engine.LoadPolicy(map[string]string{
		"compute:migrate": "role:admin and project_id:%(target.project_id)s",
	})

	assert.True(t, engine.Enforce("compute:migrate",
		map[string]interface{}{"project_id": "proj-123"},
		map[string]interface{}{"roles": []string{"admin"}, "project_id": "proj-123"},
	))
	assert.False(t, engine.Enforce("compute:migrate",
		map[string]interface{}{"project_id": "proj-456"},
		map[string]interface{}{"roles": []string{"admin"}, "project_id": "proj-123"},
	))
	assert.False(t, engine.Enforce("compute:migrate",
		map[string]interface{}{"project_id": "proj-123"},
		map[string]interface{}{"roles": []string{"member"}, "project_id": "proj-123"},
	))
}

func TestNotOperator(t *testing.T) {
	engine := NewEngine()
	engine.LoadPolicy(map[string]string{
		"deny_guest": "not role:guest",
	})

	assert.True(t, engine.Enforce("deny_guest", nil, map[string]interface{}{
		"roles": []string{"member"},
	}))
	assert.False(t, engine.Enforce("deny_guest", nil, map[string]interface{}{
		"roles": []string{"guest"},
	}))
}

func TestUnknownRuleDenies(t *testing.T) {
	engine := NewEngine()
	engine.LoadPolicy(map[string]string{})

	assert.False(t, engine.Enforce("nonexistent:action", nil, map[string]interface{}{
		"roles": []string{"admin"},
	}))
}

func TestCacheHit(t *testing.T) {
	engine := NewEngine()
	engine.LoadPolicy(map[string]string{
		"test:action": "role:admin",
	})

	creds := map[string]interface{}{"roles": []string{"admin"}}
	assert.True(t, engine.Enforce("test:action", nil, creds))
	// Second call should use cache
	assert.True(t, engine.Enforce("test:action", nil, creds))
}

func TestParentheses(t *testing.T) {
	engine := NewEngine()
	engine.LoadPolicy(map[string]string{
		"complex": "(role:admin or role:operator) and project_id:%(target.project_id)s",
	})

	assert.True(t, engine.Enforce("complex",
		map[string]interface{}{"project_id": "p1"},
		map[string]interface{}{"roles": []string{"operator"}, "project_id": "p1"},
	))
	assert.False(t, engine.Enforce("complex",
		map[string]interface{}{"project_id": "p1"},
		map[string]interface{}{"roles": []string{"member"}, "project_id": "p1"},
	))
}
