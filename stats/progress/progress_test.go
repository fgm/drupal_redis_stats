package progress

import (
	"strings"
	"testing"
)

func TestRender(t *testing.T) {
	const width = 12 // A bar of 10 characters between its brackets.
	tests := [...]struct {
		name     string
		current  float64
		expected int
	}{
		{"start", 0, 0},
		{"half", 50, 5},
		{"end", 100, 10},
		{"past the end", 150, 10},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pb := MakeProgressBar(width, 100)
			if actual := strings.Count(pb.Render(test.current), "="); actual != test.expected {
				t.Errorf("got %d bar characters, expected %d", actual, test.expected)
			}
		})
	}
}

func TestRenderEmptyDatabase(t *testing.T) {
	pb := MakeProgressBar(12, 0)
	if strings.Contains(pb.Render(0), "=") {
		t.Error("expected no bar for an empty database")
	}
}
