/*
This simple CLI application scans a Redis database for Drupal cache content.

It then return statistics about that instance, in plain text or JSON.
*/
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/gomodule/redigo/redis"

	"github.com/fgm/drupal_redis_stats/output"
	"github.com/fgm/drupal_redis_stats/stats"
)

func getVerboseWriter(quiet bool) io.Writer {
	if quiet {
		return io.Discard
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

// open the Redis connection and authenticate if needed.
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

// warn reports the keys the scan could not account for.
func warn(l *log.Logger, cs *stats.CacheStats) {
	switch n := cs.Unrecognized; {
	case n == 1:
		l.Print("skipped 1 key matching no known Drupal format")
	case n > 1:
		l.Printf("skipped %d keys matching no known Drupal format", n)
	}
	if cs.MatchedKeys()+cs.Unrecognized > 0 {
		return
	}
	switch n := cs.TotalKeys; n {
	case 0:
		l.Print("the database is empty")
	case 1:
		l.Printf(`the only key in the database does not match "%s"`, cs.MatchPattern())
	default:
		l.Printf(`none of the %d keys in the database match "%s"`, n, cs.MatchPattern())
	}
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
	prefix := fs.String("prefix", "", "Drupal cache_prefix setting: its string form, or its 'default' entry. Empty for the redis module default.")
	fs.BoolVar(&quiet, "q", false, "Do not display scan progress")
	if err := fs.Parse(os.Args[1:]); err != nil {
		log.Fatalf("failed parsing flags: %v", err)
	}

	verboseWriter := getVerboseWriter(quiet)

	if user, pass, err = getCredentials(fs, os.Stderr, *dsn, *flagUser, *flagPass); err != nil {
		log.Fatalf("failed obtaining user/pass: %v", err)
	}

	c, err := open(dsn, user, pass, fs)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	stats := stats.CacheStats{Prefix: *prefix}
	if err = stats.Scan(c, 0, verboseWriter); err != nil {
		log.Fatalf("failed SCAN: %v", err)
	}
	warn(log.Default(), &stats)

	if *jsonOutput {
		_ = output.JSON(os.Stdout, &stats)
	} else {
		output.Text(os.Stdout, &stats)
	}
}
