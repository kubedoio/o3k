package scs_test

import (
	"strings"
	"testing"

	"github.com/cobaltcore-dev/o3k/pkg/scs"
)

// TestImageSpecs_ManifestLoads is the smoke test: the embedded YAML parses
// and the canonical entries we depend on (Ubuntu 22.04 mandatory, ubuntu-capi
// recommended) are present with the upstream prefixes.
func TestImageSpecs_ManifestLoads(t *testing.T) {
	specs := scs.ImageSpecs()
	if len(specs) == 0 {
		t.Fatal("expected non-empty SCS-0104 image manifest")
	}

	var ubuntu, capi scs.ImageSpec
	for _, s := range specs {
		if s.Name == "Ubuntu 22.04" {
			ubuntu = s
		}
		if s.Name == "ubuntu-capi-image" {
			capi = s
		}
	}

	if ubuntu.Name == "" {
		t.Fatal("Ubuntu 22.04 must be in the SCS-0104 manifest")
	}
	if ubuntu.Status != scs.ImageStatusMandatory {
		t.Errorf("Ubuntu 22.04 status = %q, want mandatory", ubuntu.Status)
	}
	wantPrefix := "https://cloud-images.ubuntu.com/releases/jammy/"
	found := false
	for _, src := range ubuntu.Sources {
		if src == wantPrefix {
			found = true
		}
	}
	if !found {
		t.Errorf("Ubuntu 22.04 sources missing %q; got %v", wantPrefix, ubuntu.Sources)
	}

	if capi.Name == "" {
		t.Fatal("ubuntu-capi-image must be in the SCS-0104 manifest")
	}
	if capi.Status != scs.ImageStatusRecommended {
		t.Errorf("ubuntu-capi-image status = %q, want recommended", capi.Status)
	}
	if capi.NameScheme == "" {
		t.Error("ubuntu-capi-image must declare a name_scheme regex")
	}
}

// TestValidateImageSource_ExactNameMatch_OK is the happy path: an image whose
// name exactly matches a manifest entry, with image_source starting with one
// of the declared prefixes, is accepted.
func TestValidateImageSource_ExactNameMatch_OK(t *testing.T) {
	src := "https://cloud-images.ubuntu.com/releases/jammy/jammy-server-cloudimg-amd64.img"
	if err := scs.ValidateImageSource("Ubuntu 22.04", src); err != nil {
		t.Errorf("expected Ubuntu 22.04 with valid source to pass, got %v", err)
	}
}

// TestValidateImageSource_ExactNameMatch_BadSource is the rejection path: an
// image with an SCS-0104 known name but image_source NOT starting with any
// declared prefix must be rejected. This is the whole point of the validator
// — operators can't accidentally claim "Ubuntu 22.04" with a wrong source.
func TestValidateImageSource_ExactNameMatch_BadSource(t *testing.T) {
	src := "https://example.org/my-cooked-ubuntu.img"
	err := scs.ValidateImageSource("Ubuntu 22.04", src)
	if err == nil {
		t.Fatal("expected Ubuntu 22.04 with non-matching source to be rejected")
	}
	// Error should mention the name AND that the source didn't match a known
	// prefix, so an operator gets actionable feedback.
	if !strings.Contains(err.Error(), "Ubuntu 22.04") {
		t.Errorf("error should mention image name; got %v", err)
	}
}

// TestValidateImageSource_NameScheme_OK: ubuntu-capi-image v1.30 matches the
// recommended class's regex; with a valid source prefix it passes.
func TestValidateImageSource_NameScheme_OK(t *testing.T) {
	src := "https://nbg1.your-objectstorage.com/osism/openstack-k8s-capi-images/ubuntu-2204-kube-v1.30/foo.qcow2"
	if err := scs.ValidateImageSource("ubuntu-capi-image v1.30", src); err != nil {
		t.Errorf("expected ubuntu-capi-image v1.30 with valid source to pass, got %v", err)
	}
}

// TestValidateImageSource_NameScheme_BadSource: same name regex match, but a
// non-matching source — must be rejected.
func TestValidateImageSource_NameScheme_BadSource(t *testing.T) {
	src := "https://example.org/my-capi.qcow2"
	err := scs.ValidateImageSource("ubuntu-capi-image v1.30", src)
	if err == nil {
		t.Fatal("expected ubuntu-capi-image with non-matching source to be rejected")
	}
}

// TestValidateImageSource_UnknownName_Passthrough: an image name that doesn't
// appear in the SCS-0104 manifest passes through with no error. Operators
// remain free to publish their own images under their own names.
func TestValidateImageSource_UnknownName_Passthrough(t *testing.T) {
	if err := scs.ValidateImageSource("my-custom-image", "https://example.org/whatever.img"); err != nil {
		t.Errorf("expected unknown image name to pass through, got %v", err)
	}
}

// TestValidateImageSource_EmptySource_KnownName: an image with a known SCS
// name but NO image_source property at all — must be rejected. Operators
// claiming an SCS-known name owe the cloud an image_source.
func TestValidateImageSource_EmptySource_KnownName(t *testing.T) {
	err := scs.ValidateImageSource("Ubuntu 22.04", "")
	if err == nil {
		t.Fatal("expected empty image_source on known SCS name to be rejected")
	}
}

// TestValidateImageSource_EmptySource_UnknownName: an unknown image name with
// no source still passes through — the validator only fires on SCS-known
// names.
func TestValidateImageSource_EmptySource_UnknownName(t *testing.T) {
	if err := scs.ValidateImageSource("my-custom-image", ""); err != nil {
		t.Errorf("expected unknown name with empty source to pass through, got %v", err)
	}
}
