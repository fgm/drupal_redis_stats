/*
This simple CLI application scans a Redis database for Drupal 8 cache content.

It then return statistics about that instance, in plain text or JSON.
*/
package main

import (
	"flag"
	"io"
	"io/ioutil"
	"os"

	"github.com/gomodule/redigo/redis"

	"github.com/fgm/drupal_redis_stats/output"
	"github.com/fgm/drupal_redis_stats/stats"
)

func getLogDest(quiet bool) io.Writer {
	if quiet {
		return ioutil.Discard
	}
	return os.Stderr
}

func main() {
	dsn := flag.String("dsn", "redis://localhost:6379/0", "https://www.iana.org/assignments/uri-schemes/prov/redis")
	jsonOutput := flag.Bool("json", false, "Use JSON output.")
	quiet := flag.Bool("q", false, "Do not display scan progress")
	flag.Parse()

	// Connect to the server (ex: DB #1, default #0).
	c, err := redis.DialURL(*dsn)
	if err != nil {
		panic(err)
	}
	defer c.Close()

	logDest := getLogDest(*quiet)

	var stats stats.CacheStats
	if err = stats.Scan(c, 0, logDest); err != nil {
		panic(err)
	}

	if *jsonOutput {
		_ = output.JSON(os.Stderr, &stats)
	} else {
		output.Text(os.Stdout, &stats)
	}
}
