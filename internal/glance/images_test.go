package glance

import "testing"

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
