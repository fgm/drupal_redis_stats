package redis_test

import (
	"testing"

	"github.com/spf13/pflag"

	"github.com/fgm/drupal_redis_stats/redis"
)

func TestIsFlagPassed(t *testing.T) {
	const name = "f1"

	checks := [...]struct {
		name     string
		args     []string
		expected bool
	}{
		{"no, nil", nil, false},
		{"no, no flags", []string{}, false},
		{"no, other flags", []string{"--f2", "v2"}, false},
		{"yes, no defaultValue", []string{"--f1", ""}, true},
		{"yes, other value", []string{"--f1", "v11"}, true},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
			fs.StringP(name, "", "v1", "")
			fs.StringP("f2", "", "v2", "")
			if err := fs.Parse(check.args); err != nil {
				t.Fatalf("failed parsing: %v", err)
			}
			actual := redis.IsFlagPassed(fs, name)
			if actual != check.expected {
				t.Errorf("got %t, expected %t", actual, check.expected)
			}
		})
	}
}
