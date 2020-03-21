/*
Package output provides functions to output Drupal cache statistics.
 */
package output

import (
	"encoding/json"
	"fmt"
	"io"
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

/*
Text outputs statistics in text format for CLI usage.
 */
func Text(w io.Writer, cs *stats.CacheStats) {
	emitTextByTemplate(w, cs)
}

func emitTextByTemplate(w io.Writer, cs *stats.CacheStats) {
	name := "stats.gohtml"
	t := template.New(name)
	t.Funcs(template.FuncMap{
		"repeat": strings.Repeat,
	})
	template.Must(t.ParseFiles(name))
	err := t.Execute(w, cs)
	if err != nil {
		panic(err)
	}
}
