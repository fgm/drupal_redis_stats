package main

import (
	"flag"
	"fmt"
	"testing"
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

func TestGetLogDest(t *testing.T) {
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
			actual := getLogDest(check.quiet)
			actualType := fmt.Sprintf("%T", actual)
			if actualType != check.expectedType {
				t.Errorf("Expected %s, got %s", actualType, check.expectedType)
			}
		})
	}
}
