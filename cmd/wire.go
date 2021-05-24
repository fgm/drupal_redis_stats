// +build wireinject

package cmd

import (
	"github.com/google/wire"
	"github.com/spf13/pflag"

	"github.com/fgm/drupal_redis_stats/redis"
	"github.com/fgm/drupal_redis_stats/stats"
)

func wireAuthConn(fs *pflag.FlagSet, dsn redis.DSNValue, user redis.UserValue, pass redis.PassValue) (*redis.AuthConn, error) {
	wire.Build(redis.NewAuthConn, redis.NewBaseConn)
	return &redis.AuthConn{}, nil
}

func wireStatsScanner(fs *pflag.FlagSet, dsn redis.DSNValue, user redis.UserValue, pass redis.PassValue) (stats.Scanner, error) {
	wire.Build(stats.NewScanner, redis.NewAuthConn, redis.NewBaseConn)
	return &stats.RealScanner{}, nil
}
