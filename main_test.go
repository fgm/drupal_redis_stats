package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/fgm/drupal_redis_stats/internal/redistest"
	"github.com/fgm/drupal_redis_stats/stats"
)

func TestMain(m *testing.M) {
	os.Exit(redistest.Main(m))
}

func TestIsFlagPassed(t *testing.T) {
	const name = "f1"

	checks := [...]struct {
		name     string
		args     []string
		expected bool
	}{
		{"no, nil", nil, false},
		{"no, no flags", []string{}, false},
		{"no, other flags", []string{"-f2", "v2"}, false},
		{"yes, no defaultValue", []string{"-f1", ""}, true},
		{"yes, other value", []string{"-f1", "v11"}, true},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			fs := flag.NewFlagSet("test", flag.ContinueOnError)
			fs.String(name, "v1", "")
			fs.String("f2", "v2", "")
			if err := fs.Parse(check.args); err != nil {
				t.Fatalf("failed parsing: %v", err)
			}
			actual := isFlagPassed(fs, name)
			if actual != check.expected {
				t.Errorf("got %t, expected %t", actual, check.expected)
			}
		})
	}
}

func TestGetVerboseWriter(t *testing.T) {
	checks := [...]struct {
		name         string
		quiet        bool
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

func TestWarn(t *testing.T) {
	tests := [...]struct {
		name     string
		stats    stats.CacheStats
		expected string
	}{
		{"all matched", stats.CacheStats{TotalKeys: 5, Stats: map[string]stats.BinStats{"render": {Keys: 1}}}, ""},
		{"timestamps only", stats.CacheStats{TotalKeys: 5, Timestamps: stats.BinStats{Keys: 1}}, ""},
		{"one unrecognized", stats.CacheStats{TotalKeys: 5, Stats: map[string]stats.BinStats{"render": {Keys: 1}}, Unrecognized: 1}, "skipped 1 key matching no known Drupal format\n"},
		{"only unrecognized", stats.CacheStats{TotalKeys: 5, Unrecognized: 2}, "skipped 2 keys matching no known Drupal format\n"},
		{"one vanished", stats.CacheStats{TotalKeys: 5, Stats: map[string]stats.BinStats{"render": {Keys: 1}}, Vanished: 1}, "skipped 1 key deleted or expired during the scan\n"},
		{"only vanished", stats.CacheStats{TotalKeys: 5, Vanished: 2}, "skipped 2 keys deleted or expired during the scan\n"},
		{"empty database", stats.CacheStats{}, "the database is empty\n"},
		{"one key, not matched", stats.CacheStats{TotalKeys: 1}, `the only key in the database does not match "drupal.redis.*"` + "\n"},
		{"nothing matched, default prefix", stats.CacheStats{TotalKeys: 5}, `none of the 5 keys in the database match "drupal.redis.*"` + "\n"},
		{"nothing matched, custom prefix", stats.CacheStats{TotalKeys: 5, Prefix: "my*site"}, `none of the 5 keys in the database match "my\*site:*"` + "\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			w := strings.Builder{}
			warn(log.New(&w, "", 0), &test.stats)
			if actual := w.String(); actual != test.expected {
				t.Errorf("got %q, expected %q", actual, test.expected)
			}
		})
	}
}
