package main

import (
	"flag"
	"fmt"

	"github.com/gomodule/redigo/redis"
)

// AuthConn is a redis.Conn which is is automatically authenticated if needed.
type AuthConn struct {
	redis.Conn
}

// newAuthConn authenticates the passed redis.Conn if needed.
func newAuthConn(baseConn redis.Conn, fs *flag.FlagSet, user userValue, pass passValue) (*AuthConn, error) {
	ac := AuthConn{Conn: baseConn}
	var err error

	if user != "" || pass != "" {
		if err = authenticate(ac, isFlagPassed(fs, "user"), user, pass); err != nil {
			return nil, err
		}
	}
	return &ac, err
}

func newBaseConn(dsn dsnValue) (redis.Conn, error) {
	return redis.DialURL(string(dsn))
}

func authenticate(c redis.Conn, includeUser bool, user userValue, pass passValue) error {
	var err error

	if includeUser {
		_, err = c.Do("AUTH", user, pass)
	} else {
		_, err = c.Do("AUTH", pass)
	}
	if err != nil {
		return fmt.Errorf("failed AUTH: %w", err)
	}

	return nil
}
