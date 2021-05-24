//+build wireinject

package main

import (
	"flag"

	"github.com/google/wire"
)

func wireAuthConn(fs *flag.FlagSet, dsn dsnValue, user userValue, pass passValue) (*AuthConn, error) {
	wire.Build(newAuthConn, newBaseConn)
	return &AuthConn{}, nil
}
