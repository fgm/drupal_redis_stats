package cmd

import (
	"fmt"
	"testing"

	"github.com/fgm/drupal_redis_stats/redis"
)

func TestGetVerboseWriter(t *testing.T) {
	checks := [...]struct {
		name         string
		quiet        redis.QuietValue
		expectedType string
	}{
		{"true", true, "io.discard"},
		{"false", false, "*os.File"},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			actual := getVerboseWriter(check.quiet)
			actualType := fmt.Sprintf("%T", actual)
			if actualType != check.expectedType {
				t.Errorf("Expected %s, got %s", actualType, check.expectedType)
			}
		})
	}
}
