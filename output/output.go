/*
Package output provides functions to output Drupal cache statistics.
*/
package output

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"
	"text/template"

	"github.com/fgm/drupal_redis_stats/stats"
)

/*
JSON outputs statistics in JSON format for API usage.
*/
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

/*
Text outputs statistics in text format for CLI usage.
*/
func Text(w io.Writer, cs *stats.CacheStats) {
	if cs == nil {
		panic(errors.New("unexpected nil stats"))
	}
	const pkg = "output"
	const name = "stats.go.gotext"
	t := template.New(name)
	t.Funcs(template.FuncMap{
		"repeat": strings.Repeat,
	})
	template.Must(t.ParseFiles(
		pkg+"/"+name,
		pkg+"/hr.go.gotext",
	))

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

	err := t.Execute(w, data)
	if err != nil {
		// No failure expected for any data, so let's panic.
		panic(err)
	}
}
