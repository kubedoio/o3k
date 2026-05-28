// Package middleware — CADF audit logging.
//
// SCS audit-logging requirement (see docs/scs-alignment.md): every
// authenticated mutating request emits a structured CADF
// (Cloud Auditing Data Federation, DMTF DSP0262) event so that operators can
// reconstruct who-did-what-to-which-resource without trawling raw access logs.
//
// Events are emitted as tagged zerolog lines with `audit_event=true`. A log
// shipper (fluent-bit, vector, …) can filter on that tag and route to a SIEM.
// We intentionally do NOT introduce an audit_events DB table in this slice —
// the log stream IS the audit trail, matching O3K's broader "no message queue,
// synchronous ops" philosophy. A persisted table can follow if pilots demand
// searchable history.
package middleware

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// cadfAction maps an HTTP method to a CADF action verb. CADF defines a fixed
// taxonomy; we cover the subset relevant for OpenStack control-plane traffic.
// GET is intentionally excluded — read events are noise at the volume the API
// produces, and SCS audit guidance only requires mutations.
func cadfAction(method string) string {
	switch method {
	case "POST":
		return "create"
	case "PUT", "PATCH":
		return "update"
	case "DELETE":
		return "delete"
	default:
		return ""
	}
}

// cadfOutcome derives a CADF outcome from an HTTP status code. 2xx → success,
// everything else → failure. CADF also defines "pending" but O3K is fully
// synchronous so we never emit it.
func cadfOutcome(status int) string {
	if status >= 200 && status < 300 {
		return "success"
	}
	return "failure"
}

// targetTypeURI extracts the CADF target.typeURI from a request path. The URI
// follows OpenStack CADF conventions: `<service>/<resource>` (e.g.
// `compute/server`, `network/security_group`). Best-effort parsing — falls
// back to the raw path on anything unexpected so we never drop an audit event.
func targetTypeURI(path string) string {
	parts := pathSegments(path)
	if len(parts) == 0 {
		return path
	}

	// Walk segments looking for the first one that maps to a known service.
	// Nova/Cinder paths interpose an opaque project ID between the version and
	// the resource collection (e.g. /v2.1/<project-id>/servers); Keystone and
	// Neutron put the collection directly after the version. Walking lets us
	// handle both without needing to recognise project IDs by shape.
	for i, seg := range parts {
		if service := cadfServiceForResource(seg); service != "" {
			return service + "/" + singular(seg)
		}
		_ = i
	}
	return path
}

// pathSegments splits a request path and strips the leading API version
// segment if present. Recognises every version family the o3k services
// expose: Keystone /v3, Nova /v2.1, Neutron /v2.0, Glance /v2, Cinder /v3.
func pathSegments(path string) []string {
	trimmed := strings.TrimPrefix(path, "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) > 0 && isVersionSegment(parts[0]) {
		parts = parts[1:]
	}
	return parts
}

// isVersionSegment matches any OpenStack-style version prefix (`v3`, `v2.1`,
// `v2.0`, `v2`, `v1`). Conservative — only strips actual version segments,
// never collections that happen to start with "v".
func isVersionSegment(s string) bool {
	if len(s) < 2 || s[0] != 'v' {
		return false
	}
	for _, r := range s[1:] {
		if (r < '0' || r > '9') && r != '.' {
			return false
		}
	}
	return true
}

// cadfServiceForResource maps a REST collection name to its CADF service
// namespace. Covers the five core services O3K exposes.
func cadfServiceForResource(collection string) string {
	switch collection {
	case "servers", "flavors", "keypairs", "os-keypairs", "os-hypervisors":
		return "compute"
	case "networks", "subnets", "ports", "routers", "security-groups", "floatingips":
		return "network"
	case "volumes", "snapshots", "types", "backups":
		return "block-storage"
	case "images":
		return "image"
	case "users", "projects", "roles", "domains", "auth", "tokens", "groups", "credentials":
		return "identity"
	case "resource_providers", "allocations", "inventories":
		return "placement"
	default:
		return ""
	}
}

// singular converts a REST collection name to its CADF resource singular.
// Handles the irregular cases the o3k API uses; trailing-s is the fallback.
func singular(s string) string {
	switch s {
	case "security-groups":
		return "security_group"
	case "os-keypairs", "keypairs":
		return "keypair"
	case "floatingips":
		return "floatingip"
	case "resource_providers":
		return "resource_provider"
	case "inventories":
		return "inventory"
	case "policies":
		return "policy"
	}
	if strings.HasSuffix(s, "ies") {
		return strings.TrimSuffix(s, "ies") + "y"
	}
	if strings.HasSuffix(s, "s") {
		return strings.TrimSuffix(s, "s")
	}
	return s
}

// targetID extracts the resource UUID from the path when the URL targets a
// specific resource (e.g. DELETE /v3/servers/<id>). Returns empty string for
// collection-level operations. Looks for a UUID *after* a known resource
// collection so that project-scope UUIDs (Nova/Cinder) aren't mistaken for
// the target.
func targetID(path string) string {
	parts := pathSegments(path)
	for i, seg := range parts {
		if cadfServiceForResource(seg) == "" {
			continue
		}
		if i+1 < len(parts) && isUUID(parts[i+1]) {
			return parts[i+1]
		}
		return ""
	}
	return ""
}

// isUUID is a cheap shape check — full UUID parsing is overkill for path
// segment classification. 36 chars with dashes at positions 8/13/18/23.
func isUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	return s[8] == '-' && s[13] == '-' && s[18] == '-' && s[23] == '-'
}

// AuditMiddleware emits a CADF event for every authenticated mutating request.
// Mounts AFTER AuthMiddleware so user_id/project_id/user_name are available in
// context. Read-only requests (GET, HEAD, OPTIONS) are skipped to keep the
// stream signal-rich.
func AuditMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		action := cadfAction(c.Request.Method)
		if action == "" {
			c.Next()
			return
		}

		c.Next()

		userID, _ := c.Get("user_id")
		userName, _ := c.Get("user_name")
		projectID, _ := c.Get("project_id")

		path := c.Request.URL.Path
		event := log.Info().
			Bool("audit_event", true).
			Str("eventType", "activity").
			Str("id", uuid.New().String()).
			Str("eventTime", time.Now().UTC().Format(time.RFC3339Nano)).
			Str("action", action).
			Str("outcome", cadfOutcome(c.Writer.Status())).
			Str("initiator.id", asString(userID)).
			Str("initiator.name", asString(userName)).
			Str("initiator.project_id", asString(projectID)).
			Str("initiator.host.address", c.ClientIP()).
			Str("target.typeURI", targetTypeURI(path)).
			Str("observer.id", "o3k").
			Str("observer.typeURI", "service/security").
			Str("requestPath", path).
			Int("reason.reasonCode", c.Writer.Status())

		if tid := targetID(path); tid != "" {
			event = event.Str("target.id", tid)
		}
		if rid := c.GetString("request_id"); rid != "" {
			event = event.Str("request_id", rid)
		}

		event.Msg("cadf audit event")
	}
}

// asString safely coerces a context value to a string. Missing or non-string
// values become empty — CADF allows empty initiator fields for unauthenticated
// flows but the AuditMiddleware mount point ensures auth has already run.
func asString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
