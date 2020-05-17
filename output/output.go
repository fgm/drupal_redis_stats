/*
Package output provides functions to output Drupal cache statistics.
*/
package output

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"strings"
	"text/template"

	"github.com/markbates/pkger"

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
compileTemplates loads and parses the template, either from the file system in
development mode, or from the embedded version in production mode.
*/
func compileTemplates() (*template.Template, error) {
	templateDir := pkger.Include("/output/templates")

	tpl := template.New("")
	tpl.Funcs(template.FuncMap{
		"repeat": strings.Repeat,
	})

	for _, name := range []string{"hr", "stats"} {
		resource, err := pkger.Open(strings.Join([]string{
			templateDir,
			name + ".go.gotext",
		}, "/"))
		if err != nil {
			return nil, fmt.Errorf("failed opening %s template: %w", name, err)
		}
		contents, _ := ioutil.ReadAll(resource)
		if _, err = tpl.Parse(string(contents)); err != nil {
			return nil, fmt.Errorf("failed parsing %s template: %w", name, err)
		}
	}
	return tpl, nil
}

/*
Text outputs statistics in text format for CLI usage.
*/
func Text(w io.Writer, cs *stats.CacheStats) {
	if cs == nil {
		panic(errors.New("unexpected nil stats"))
	}
	tpl, _ := compileTemplates()

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

	err := tpl.Execute(w, data)
	if err != nil {
		// No failure expected for any data, so let's panic.
		panic(err)
	}
}
