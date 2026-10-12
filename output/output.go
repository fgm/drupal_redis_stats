// Package output provides functions to output Drupal cache statistics.
package output

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"
	"text/template"
	"unicode/utf8"

	"github.com/fgm/drupal_redis_stats/stats"
)

//go:embed templates/hr.go.gotext
var tplHr string

//go:embed templates/stats.go.gotext
var tplStats string

// JSON outputs statistics in JSON format for API usage.
func JSON(w io.Writer, stats *stats.CacheStats) error {
	// The CacheStats type cannot fail serialization.
	j, _ := json.Marshal(stats)
	fmt.Fprintf(w, "%s\n", j)
	return nil
}

// row is a non-bin category shown between the bins and the total.
type row struct {
	Label string
	Stats stats.BinStats
}

type templateData struct {
	Stats                              *stats.CacheStats
	Others                             []row
	BinsHeader, KeysHeader, SizeHeader string
	BinsFooter                         string
	BinsLen, KeysLen, SizeLen          int
}

// compileTemplates loads and parses the template, either from the file system in
// development mode, or from the embedded version in production mode.
func compileTemplates() (*template.Template, error) {
	tpl := template.New("")
	tpl.Funcs(template.FuncMap{
		"repeat": strings.Repeat,
	})

	for _, contents := range []string{tplHr, tplStats} {
		tpl = template.Must(tpl.Parse(contents))
	}
	return tpl, nil
}

// Text outputs statistics in text format for CLI usage.
func Text(w io.Writer, cs *stats.CacheStats) {
	if cs == nil {
		panic(errors.New("unexpected nil stats"))
	}
	tpl, _ := compileTemplates()

	const binsHeader = "Bin"
	const keysHeader = "Keys"
	const sizeHeader = "Data"
	const binsFooter = "Total"
	binsLen := max(cs.MaxBinNameLength(), utf8.RuneCountInString(binsFooter))
	var others []row
	for _, r := range []row{
		{"(cache tags)", cs.CacheTags},
		{"(flood)", cs.Flood},
		{"(locks)", cs.Locks},
		{"(timestamps)", cs.Timestamps},
	} {
		if r.Stats.Keys > 0 {
			others = append(others, r)
			binsLen = max(binsLen, utf8.RuneCountInString(r.Label))
		}
	}
	data := templateData{
		Stats:      cs,
		Others:     others,
		BinsHeader: binsHeader,
		KeysHeader: keysHeader,
		SizeHeader: sizeHeader,
		BinsFooter: binsFooter,
		BinsLen:    binsLen,
		KeysLen:    int(math.Max(float64(cs.ItemCountLength()), float64(len(keysHeader)))),
		SizeLen:    int(math.Max(float64(cs.TotalSizeLength()), float64(len(sizeHeader)))),
	}

	err := tpl.Execute(w, data)
	if err != nil {
		// No failure expected for any data, so let's panic.
		panic(err)
	}
}
