// Package output provides functions to output Drupal cache statistics.
package output

import (
	_ "embed" // Imported for templates embedding.
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"
	"text/template"

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

type templateData struct {
	Stats                              *stats.CacheStats
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
func Text(w io.Writer, cs *stats.CacheStats) error {
	if cs == nil {
		return errors.New("unexpected nil stats")
	}
	tpl, err := compileTemplates()
	if err != nil {
		return err
	}

	const binsHeader = "Bin"
	const keysHeader = "Keys"
	const sizeHeader = "Data"
	const binsFooter = "Total"
	data := templateData{
		Stats:      cs,
		BinsHeader: binsHeader,
		KeysHeader: keysHeader,
		SizeHeader: sizeHeader,
		BinsFooter: binsFooter,
		BinsLen:    int(math.Max(float64(cs.MaxBinNameLength()), float64(len(binsFooter)))),
		KeysLen:    int(math.Max(float64(cs.ItemCountLength()), float64(len(keysHeader)))),
		SizeLen:    int(math.Max(float64(cs.TotalSizeLength()), float64(len(sizeHeader)))),
	}

	err = tpl.Execute(w, data)
	if err != nil {
		return err
	}
	return nil
}
