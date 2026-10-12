package redistest

import (
	"strings"
	"testing"
)

// TestImagesIncludeSampling checks that the tests run against a version
// sampling hash fields by default: only on such a version can a test show
// that sizes are measured with SAMPLES 0.
func TestImagesIncludeSampling(t *testing.T) {
	for _, image := range Images {
		_, tag, _ := strings.Cut(image, ":")
		major, minor, err := parseVersion(tag)
		if err != nil {
			t.Fatalf("image %s: %v", image, err)
		}
		if SamplesHashFields(major, minor) {
			return
		}
	}
	t.Errorf("no image in %v samples hash fields by default", Images)
}

func TestSamplesHashFields(t *testing.T) {
	checks := [...]struct {
		version  string
		expected bool
	}{
		{"7.4.11", true},
		{"8.2.3", true},
		{"8.4.0", false},
		{"8.10.2", false},
		{"9.0.0", false},
	}
	for _, check := range checks {
		t.Run(check.version, func(t *testing.T) {
			major, minor, err := parseVersion(check.version)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if actual := SamplesHashFields(major, minor); actual != check.expected {
				t.Errorf("got %t, expected %t", actual, check.expected)
			}
		})
	}
}

func TestParseVersionErrors(t *testing.T) {
	for _, version := range [...]string{"", "8", "x.4", "8.x", "latest"} {
		if _, _, err := parseVersion(version); err == nil {
			t.Errorf("parsing %q: expected an error", version)
		}
	}
}
