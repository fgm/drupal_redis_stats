/*
This simple CLI application scans a Redis database for Drupal 8 cache content.

It then return statistics about that instance, in plain text or JSON.
*/
package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"

	"github.com/gomodule/redigo/redis"

	"github.com/fgm/drupal_redis_stats/output"
	"github.com/fgm/drupal_redis_stats/stats"
)

func getVerboseWriter(quiet bool) io.Writer {
	if quiet {
		return ioutil.Discard
	}
	return os.Stderr
}

func isFlagPassed(fs *flag.FlagSet, name string) bool {
	found := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

// open the Redis connection and authenticate is needed.
func open(dsn *string, user string, pass string, fs *flag.FlagSet) (redis.Conn, error) {
	var c redis.Conn
	var err error

	// Connect to the server (ex: DB #1, default #0).
	if c, err = redis.DialURL(*dsn); err != nil {
		return nil, fmt.Errorf("failed dialing Redis URL: %w", err)
	}

	if user != "" || pass != "" {
		if err = authenticate(c, isFlagPassed(fs, "user"), user, pass); err != nil {
			return nil, err
		}
	}
	return c, err
}

func main() {
	var user, pass string
	var err error
	var quiet bool

	fs := flag.NewFlagSet("cli", flag.ContinueOnError)
	flagUser := fs.String("user", "", "user name if Redis is configured with ACL. Overrides the DSN user.")
	flagPass := fs.String("pass", "", "Password. If it is empty it's asked from the tty. Overrides the DSN password.")
	dsn := fs.String("dsn", "redis://localhost:6379/0", "Can include user and password, per https://www.iana.org/assignments/uri-schemes/prov/redis")
	jsonOutput := fs.Bool("json", false, "Use JSON output.")
	fs.BoolVar(&quiet, "q", false, "Do not display scan progress")
	if err := fs.Parse(os.Args[1:]); err != nil {
		log.Fatalf("failed parsing flags: %v", err)
	}

	verboseWriter := getVerboseWriter(quiet)

	if user, pass, err = getCredentials(fs, os.Stdout, *dsn, *flagUser, *flagPass); err != nil {
		log.Fatalf("failed obtaining user/pass: %v", err)
	}

	c, err := open(dsn, user, pass, fs)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	var stats stats.CacheStats
	if err = stats.Scan(c, 0, verboseWriter); err != nil {
		log.Fatalf("failed SCAN: %v", err)
	}

	if *jsonOutput {
		_ = output.JSON(os.Stderr, &stats)
	} else {
		output.Text(os.Stdout, &stats)
	}
}
