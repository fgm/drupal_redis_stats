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
			t.Errorf("Did not find expected value %s for %s", expected, expectation.name)
		}
	}
}

func TestTextSadNil(t *testing.T) {
	w := strings.Builder{}
	var s *stats.CacheStats

	recovered := func() (r bool) {
		defer func() {
			if panicked := recover(); panicked != nil {
				r = true
			}
		}()
		output.Text(&w, s)
		return r
	}
	if !recovered() {
		t.Errorf("test did not panic on nil stats")
	}
}

func TestTextSadWriter(t *testing.T) {
	defer func() {
		if err := recover(); err == nil {
			t.Error("calling Text() did not cause panic")
		}
	}()
	output.Text(ErrorWriter(0), &stats.CacheStats{})
}

func TestText(t *testing.T) {
	w := strings.Builder{}
	var s = &sampleStats
	output.Text(&w, s)
	actual := w.String()
	for _, expectation := range sampleExpectations {
		expected := strconv.Itoa(expectation.value)
		if pos := strings.Index(actual, expected); pos <= 0 {
			t.Errorf("Did not find expected value %s for %s", expected, expectation.name)
		}
	}
}

func TestTimestamps(t *testing.T) {
	const label = "(timestamps)"
	withTimestamps := sampleStats
	withTimestamps.Timestamps = stats.BinStats{Keys: 3, Size: 168}

	tests := [...]struct {
		name     string
		stats    *stats.CacheStats
		expected bool
	}{
		{"absent", &sampleStats, false},
		{"present", &withTimestamps, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			text := strings.Builder{}
			output.Text(&text, test.stats)
			if actual := strings.Contains(text.String(), label); actual != test.expected {
				t.Errorf("text has timestamps row: %t, expected %t:\n%s", actual, test.expected, text.String())
			}
			j := strings.Builder{}
			_ = output.JSON(&j, test.stats)
			if actual := strings.Contains(j.String(), `"Timestamps":{"Keys":3,"Size":168}`); actual != test.expected {
				t.Errorf("JSON has timestamps: %t, expected %t: %s", actual, test.expected, j.String())
			}
			if !test.expected && strings.Contains(j.String(), "Timestamps") {
				t.Errorf("JSON has an empty timestamps field: %s", j.String())
			}
		})
	}
}

func TestJSONOtherCategories(t *testing.T) {
	cs := stats.CacheStats{CacheTags: stats.BinStats{Keys: 1, Size: 2}, Locks: stats.BinStats{Keys: 3, Size: 4}}
	w := strings.Builder{}
	_ = output.JSON(&w, &cs)
	j := w.String()
	for _, expected := range []string{`"CacheTags":{"Keys":1,"Size":2}`, `"Locks":{"Keys":3,"Size":4}`} {
		if !strings.Contains(j, expected) {
			t.Errorf("missing %s in %s", expected, j)
		}
	}
	if strings.Contains(j, "Flood") {
		t.Errorf("JSON has an empty Flood field: %s", j)
	}
}

func TestTextLayout(t *testing.T) {
	bins := map[string]stats.BinStats{"render": {Keys: 3, Size: 150}, "data": {Keys: 1, Size: 20}}
	tests := [...]struct {
		name     string
		stats    stats.CacheStats
		expected string
	}{
		{"no timestamps", stats.CacheStats{TotalKeys: 4, Stats: bins}, `Bin    | Keys | Data
-------+------+-----
data   |    1 |   20
render |    3 |  150
-------+------+-----
Total  |    4 |  170
`},
		{"other categories, label wider than bins", stats.CacheStats{TotalKeys: 14, Stats: bins,
			CacheTags: stats.BinStats{Keys: 5, Size: 500}, Locks: stats.BinStats{Keys: 2, Size: 80}, Timestamps: stats.BinStats{Keys: 2, Size: 1000}}, `Bin          | Keys | Data
-------------+------+-----
data         |    1 |   20
render       |    3 |  150
-------------+------+-----
(cache tags) |    5 |  500
(locks)      |    2 |   80
(timestamps) |    2 | 1000
-------------+------+-----
Total        |   14 | 1750
`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			w := strings.Builder{}
			output.Text(&w, &test.stats)
			if actual := w.String(); actual != test.expected {
				t.Errorf("got:\n%s\nexpected:\n%s", actual, test.expected)
			}
		})
	}
}

func TestTextTimestampsOnly(t *testing.T) {
	const row = "(timestamps) |    2 |  160\n"
	w := strings.Builder{}
	output.Text(&w, &stats.CacheStats{TotalKeys: 2, Stats: map[string]stats.BinStats{}, Timestamps: stats.BinStats{Keys: 2, Size: 160}})
	if !strings.Contains(w.String(), row) {
		t.Errorf("missing timestamps row %q in:\n%s", row, w.String())
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
