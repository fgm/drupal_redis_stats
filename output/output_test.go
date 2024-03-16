package output_test

import (
	"os"
	"path"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/fgm/drupal_redis_stats/output"
	"github.com/fgm/drupal_redis_stats/stats"
)

var sampleStats = stats.CacheStats{
	TotalKeys: 25,
	Stats: map[string]stats.BinStats{
		"default": {
			Keys: 12,
			Size: 37,
		},
		"form": {
			Keys: 13,
			Size: 22,
		},
	},
}

var sampleExpectations = []struct {
	value int
	name  string
}{
	{25, "TotalKeys"},
	{12, "default keys"},
	{37, "default size"},
	{13, "form keys"},
	{22, "form size"},
}

func TestJSON(t *testing.T) {
	w := strings.Builder{}
	var s *stats.CacheStats

	err := output.JSON(&w, s)
	if err != nil {
		t.Errorf("nil stats should not fail serialization")
	}
	actual := w.String()
	if actual != "null\n" {
		t.Errorf(`nil stats should serialize as "null", got %#v`, actual)
	}

	s = &sampleStats
	err = output.JSON(&w, s)
	if err != nil {
		t.Errorf("non-nil stats should pass serialization")
	}
	j := w.String()
	for _, expectation := range sampleExpectations {
		expected := strconv.Itoa(expectation.value)
		if pos := strings.Index(j, expected); pos <= 0 {
			t.Errorf("Did not find expected value %sampleStats for %sampleStats", expected, expectation.name)
		}
	}
}

func TestTextSadNil(t *testing.T) {
	w := strings.Builder{}
	var s *stats.CacheStats

	if err :=output.Text(&w, s); err == nil {
		t.Errorf("test did not error on nil stats")
	}
}

func TestTextSadWriter(t *testing.T) {
	if err := output.Text(ErrorWriter(0), &stats.CacheStats{}); err == nil {
		t.Error("unexpected success")
	}
}

func TestText(t *testing.T) {
	w := strings.Builder{}
	var s = &sampleStats
	output.Text(&w, s)
	actual := w.String()
	for _, expectation := range sampleExpectations {
		expected := strconv.Itoa(expectation.value)
		if pos := strings.Index(actual, expected); pos <= 0 {
			t.Errorf("Did not find expected value %sampleStats for %sampleStats", expected, expectation.name)
		}
	}
}

func BenchmarkText(b *testing.B) {
	w := strings.Builder{}
	var s = &sampleStats
	for n := 0; n < b.N; n++ {
		output.Text(&w, s)
		w.Reset()
	}
}

func init() {
	// Filename will be the absolute path to this very file, however the test is run.
	// Credit: https://brandur.org/fragments/testing-go-project-root
	_, filename, _, _ := runtime.Caller(0)
	dir := path.Join(path.Dir(filename), "..")
	err := os.Chdir(dir)
	if err != nil {
		panic(err)
	}
}
