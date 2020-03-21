/*
Package output provides functions to output Drupal cache statistics.
*/
package output

import (
	"encoding/json"
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
func JSON(w io.Writer, stats *stats.CacheStats) {
	j, err := json.Marshal(stats)
	if err != nil {
		panic(err)
	}
	fmt.Fprintf(w, "%s\n", j)
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
	name := "stats.go.gotext"
	t := template.New(name)
	t.Funcs(template.FuncMap{
		"repeat": strings.Repeat,
	})
	template.Must(t.ParseFiles(
		"output/"+name,
		"output/hr.go.gotext",
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
		panic(err)
	}
}
