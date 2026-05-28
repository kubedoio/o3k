package common

import (
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Create a temporary config file
	tmpfile, err := os.CreateTemp("", "o3k-test-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	configContent := `
database:
  url: "postgres://test:test@localhost/test"
  max_connections: 10

keystone:
  port: 5000
  jwt_secret: "test-secret"
  token_ttl: 24h
  admin_user: "admin"
  admin_password: "secret"

nova:
  port: 8774
  libvirt_uri: "qemu:///system"
  default_flavor: "m1.small"

neutron:
  port: 9696
  dhcp_lease_time: 24h
  iptables_enabled: true

cinder:
  port: 8776
  ceph_pool: "volumes"
  ceph_conf: "/etc/ceph/ceph.conf"

glance:
  port: 9292
  ceph_pool: "images"
  ceph_conf: "/etc/ceph/ceph.conf"

logging:
  level: "info"
  format: "json"
`

	if _, err := tmpfile.Write([]byte(configContent)); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	tmpfile.Close()

	config, err := LoadConfig(tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify database config
	if config.Database.URL != "postgres://test:test@localhost/test" {
		t.Errorf("Expected database URL postgres://test:test@localhost/test, got %s", config.Database.URL)
	}

	if config.Database.MaxConnections != 10 {
		t.Errorf("Expected max connections 10, got %d", config.Database.MaxConnections)
	}

	// Verify keystone config
	if config.Keystone.Port != 5000 {
		t.Errorf("Expected keystone port 5000, got %d", config.Keystone.Port)
	}

	if config.Keystone.JWTSecret != "test-secret" {
		t.Errorf("Expected JWT secret test-secret, got %s", config.Keystone.JWTSecret)
	}

	if config.Keystone.AdminUser != "admin" {
		t.Errorf("Expected admin user admin, got %s", config.Keystone.AdminUser)
	}

	// Verify nova config
	if config.Nova.Port != 8774 {
		t.Errorf("Expected nova port 8774, got %d", config.Nova.Port)
	}

	if config.Nova.LibvirtURI != "qemu:///system" {
		t.Errorf("Expected libvirt URI qemu:///system, got %s", config.Nova.LibvirtURI)
	}

	// Verify neutron config
	if config.Neutron.Port != 9696 {
		t.Errorf("Expected neutron port 9696, got %d", config.Neutron.Port)
	}

	if !config.Neutron.IPTablesEnabled {
		t.Error("Expected iptables enabled")
	}

	// Verify cinder config
	if config.Cinder.Port != 8776 {
		t.Errorf("Expected cinder port 8776, got %d", config.Cinder.Port)
	}

	if config.Cinder.CephPool != "volumes" {
		t.Errorf("Expected ceph pool volumes, got %s", config.Cinder.CephPool)
	}

	// Verify glance config
	if config.Glance.Port != 9292 {
		t.Errorf("Expected glance port 9292, got %d", config.Glance.Port)
	}

	if config.Glance.CephPool != "images" {
		t.Errorf("Expected ceph pool images, got %s", config.Glance.CephPool)
	}

	// Verify logging config
	if config.Logging.Level != "info" {
		t.Errorf("Expected log level info, got %s", config.Logging.Level)
	}

	if config.Logging.Format != "json" {
		t.Errorf("Expected log format json, got %s", config.Logging.Format)
	}
}

func TestLoadConfigMissingFile(t *testing.T) {
	// A missing config file is the zero-config case — LoadConfig returns
	// an empty Config with no error so the caller can apply bootstrap defaults.
	cfg, err := LoadConfig("/nonexistent/config.yaml")
	if err != nil {
		t.Errorf("Expected nil error for missing config file (zero-config mode), got: %v", err)
	}
	if cfg == nil {
		t.Error("Expected non-nil Config for missing config file")
	}
}

func TestLoadConfigInvalidYAML(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "o3k-invalid-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	invalidYAML := `
database:
  url: "postgres://test@localhost/test"
  invalid yaml structure
    - broken
`

	if _, err := tmpfile.Write([]byte(invalidYAML)); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	tmpfile.Close()

	_, err = LoadConfig(tmpfile.Name())
	if err == nil {
		t.Error("Expected error for invalid YAML, got nil")
	}
}

// TestValidateConfigProductionStubGuard exercises the production-environment
// stub-mode refusal. When O3K_ENV=production, ValidateConfig must reject any
// service still running in stub mode — stub backends return fake data and a
// production deployment that slips into stub by config drift would silently
// lose every API call.
func TestValidateConfigProductionStubGuard(t *testing.T) {
	// Save and restore O3K_ENV so the test doesn't leak state.
	origEnv := os.Getenv("O3K_ENV")
	defer os.Setenv("O3K_ENV", origEnv)
	os.Setenv("O3K_ENV", "production")

	// nonStub is the baseline production-safe config the per-service cases
	// mutate one field at a time. Each field is set to a non-stub value that
	// passes the existing enum validation.
	nonStub := func() *Config {
		return &Config{
			Database: DatabaseConfig{Datastore: "postgres://localhost/o3k"},
			Nova:     NovaConfig{LibvirtMode: "real", LibvirtURI: "qemu:///system"},
			Neutron:  NeutronConfig{NetworkingMode: "iptables"},
			Cinder:   CinderConfig{StorageMode: "rbd"},
			Glance:   GlanceConfig{StorageMode: "rbd"},
		}
	}

	t.Run("happy path - no stub modes", func(t *testing.T) {
		if err := ValidateConfig(nonStub()); err != nil {
			t.Errorf("expected nil error for production-safe config, got: %v", err)
		}
	})

	t.Run("nova stub refused", func(t *testing.T) {
		cfg := nonStub()
		cfg.Nova.LibvirtMode = "stub"
		err := ValidateConfig(cfg)
		if err == nil {
			t.Fatal("expected error for nova.libvirt_mode=stub in production, got nil")
		}
		if !contains(err.Error(), "nova.libvirt_mode") {
			t.Errorf("expected error to mention nova.libvirt_mode, got: %v", err)
		}
	})

	t.Run("neutron stub refused", func(t *testing.T) {
		cfg := nonStub()
		cfg.Neutron.NetworkingMode = "stub"
		err := ValidateConfig(cfg)
		if err == nil {
			t.Fatal("expected error for neutron.networking_mode=stub in production, got nil")
		}
		if !contains(err.Error(), "neutron.networking_mode") {
			t.Errorf("expected error to mention neutron.networking_mode, got: %v", err)
		}
	})

	t.Run("cinder stub refused", func(t *testing.T) {
		cfg := nonStub()
		cfg.Cinder.StorageMode = "stub"
		err := ValidateConfig(cfg)
		if err == nil {
			t.Fatal("expected error for cinder.storage_mode=stub in production, got nil")
		}
		if !contains(err.Error(), "cinder.storage_mode") {
			t.Errorf("expected error to mention cinder.storage_mode, got: %v", err)
		}
	})

	t.Run("glance stub refused", func(t *testing.T) {
		cfg := nonStub()
		cfg.Glance.StorageMode = "stub"
		err := ValidateConfig(cfg)
		if err == nil {
			t.Fatal("expected error for glance.storage_mode=stub in production, got nil")
		}
		if !contains(err.Error(), "glance.storage_mode") {
			t.Errorf("expected error to mention glance.storage_mode, got: %v", err)
		}
	})

	t.Run("stub allowed when O3K_ENV unset", func(t *testing.T) {
		os.Setenv("O3K_ENV", "")
		defer os.Setenv("O3K_ENV", "production")
		cfg := nonStub()
		cfg.Nova.LibvirtMode = "stub"
		cfg.Neutron.NetworkingMode = "stub"
		cfg.Cinder.StorageMode = "stub"
		cfg.Glance.StorageMode = "stub"
		if err := ValidateConfig(cfg); err != nil {
			t.Errorf("expected stub modes to pass when O3K_ENV is unset, got: %v", err)
		}
	})

	t.Run("stub allowed when O3K_ENV=development", func(t *testing.T) {
		os.Setenv("O3K_ENV", "development")
		defer os.Setenv("O3K_ENV", "production")
		cfg := nonStub()
		cfg.Nova.LibvirtMode = "stub"
		if err := ValidateConfig(cfg); err != nil {
			t.Errorf("expected stub modes to pass in development, got: %v", err)
		}
	})
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle || indexOf(haystack, needle) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
