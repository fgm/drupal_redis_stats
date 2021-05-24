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

	"github.com/fgm/drupal_redis_stats/output"
	"github.com/fgm/drupal_redis_stats/stats"
)

func getVerboseWriter(quiet quietValue) io.Writer {
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

type (
	dsnValue   string
	jsonValue  bool
	passValue  string
	quietValue bool
	userValue  string
)

func configure(args []string) (*flag.FlagSet, dsnValue, userValue, passValue, jsonValue, quietValue, error) {
	fs := flag.NewFlagSet("cli", flag.ContinueOnError)
	flagDSN := fs.String("dsn", "redis://localhost:6379/0", "Can include user and password, per https://www.iana.org/assignments/uri-schemes/prov/redis")
	flagUser := fs.String("user", "", "user name if Redis is configured with ACL. Overrides the DSN user.")
	flagPass := fs.String("pass", "", "Password. If it is empty it's asked from the tty. Overrides the DSN password.")
	jsonOutput := fs.Bool("json", false, "Use JSON output.")
	quiet := fs.Bool("q", false, "Do not display scan progress")
	if err := fs.Parse(args); err != nil {
		log.Fatalf("failed parsing flags: %v", err)
	}

	user, pass, err := getCredentials(fs, os.Stdout, *flagDSN, *flagUser, *flagPass)
	if err != nil {
		return nil, "", "", "", false, false, fmt.Errorf("failed obtaining user/pass: %v", err)
	}

	return fs, dsnValue(*flagDSN), user, pass, jsonValue(*jsonOutput), quietValue(*quiet), nil
}

func main() {
	fs, dsn, user, pass, jsonOutput, quiet, err := configure(os.Args[1:])
	if err != nil {
		log.Fatalf("failed configuring: %v", err)
	}

	verboseWriter := getVerboseWriter(quiet)

	c, err := wireAuthConn(fs, dsn, user, pass)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	var stats stats.CacheStats
	if err = stats.Scan(c, 0, verboseWriter); err != nil {
		log.Fatalf("failed SCAN: %v", err)
	}

	if jsonOutput {
		_ = output.JSON(os.Stderr, &stats)
	} else {
		output.Text(os.Stdout, &stats)
	}
}
