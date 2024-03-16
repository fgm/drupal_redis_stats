package redis

import (
	"fmt"

	"github.com/gomodule/redigo/redis"
	"github.com/spf13/pflag"
)

// AuthConn is a redis.Conn which is is automatically authenticated if needed.
type AuthConn struct {
	redis.Conn
}

// NewAuthConn authenticates the passed redis.Conn if needed.
func NewAuthConn(baseConn redis.Conn, fs *pflag.FlagSet, user UserValue, pass PassValue) (*AuthConn, error) {
	ac := AuthConn{Conn: baseConn}
	var err error

	if user != "" || pass != "" {
		if err = Authenticate(ac, IsFlagPassed(fs, "user"), user, pass); err != nil {
			return nil, err
		}
	}
	return &ac, err
}

// NewBaseConn provides a standard Redis connection.
func NewBaseConn(dsn DSNValue) (redis.Conn, error) {
	return redis.DialURL(string(dsn))
}

// Authenticate attempts authentication on an already opened standard Redis connection.
func Authenticate(c redis.Conn, includeUser bool, user UserValue, pass PassValue) error {
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
