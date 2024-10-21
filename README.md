Drupal Redis Stats
==================

[![GoDoc](https://godoc.org/github.com/fgm/drupal_redis_stats?status.svg)](https://godoc.org/github.com/fgm/drupal_redis_stats)
[![Go Report Card](https://goreportcard.com/badge/github.com/fgm/drupal_redis_stats)](https://goreportcard.com/report/github.com/fgm/drupal_redis_stats)
[![Build Status](https://travis-ci.org/fgm/drupal_redis_stats.svg?branch=master)](https://travis-ci.org/fgm/drupal_redis_stats)
[![codecov](https://codecov.io/gh/fgm/drupal_redis_stats/branch/main/graph/badge.svg?token=QR0XKBK3DF)](https://codecov.io/gh/fgm/drupal_redis_stats)
[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/fgm/drupal_redis_stats/badge)](https://securityscorecards.dev/viewer/?uri=github.com/fgm/drupal_redis_stats)
[![FOSSA Status](https://app.fossa.com/api/projects/custom%2B11916%2Fgithub.com%2Ffgm%2Fdrupal_redis_stats.svg?type=shield)](https://app.fossa.com/projects/custom%2B11916%2Fgithub.com%2Ffgm%2Fdrupal_redis_stats?ref=badge_shield)

This command provides a summary of the use of a Redis database by the 
Drupal 10 or Drupal 9 cache provider.

It relies on Redis `SCAN` operator instead of `KEYS`, so it won't block your
site when used on production.


## Installing

Assuming Go 1.21 or later installed:

```
go get -u github.com/fgm/drupal_redis_stats
```


## Using
### Flags

- `-h` provides help
- `-dsn` flag allows using a non-default Redis
  - `-dsn redis://<host>:<port>/<db>` without authentication
  - `-dsn redis://<password>@<host>:<port>/<db>` for `requirepass` AUTH mode
  - `-dsn redis://<user>:<password>@<host>:<port>/<db>` for ACL AUTH mode
- `-json` provides JSON output instead of the default human-readable format
- `-q` disables the progress bar used during the database SCAN loop


### Sample results

```
Bin                | Entries |     Size
-------------------+---------+---------
bootstrap          |      10 |     6056
config             |    7992 |  4741812
data               |   10483 |  7717813
default            |     183 |   130061
discovery          |     240 |   164028
dynamic_page_cache |    6104 | 11225548
entity             |     785 |   506817
menu               |      28 |    30646
page               |    6125 |  5222326
render             |   12877 | 19965812
-------------------+---------+---------
Total              |   44832 | 49714857
```

The _Entries_ column provides the number of entries in a cache bin,
while the _Size_ bin provides the size used by keys and data in Redis
storage, based on information provided by the `MEMORY USAGE` command.

### Testing

- Run only unit tests: `make test`
- Run unit and integration tests: `make test-ci`, assuming:
  - on `localhost:6379`: an unauthenticated Redis instance 
  - on `localhost:6380`: a Redis instance with: 
    - `ACL SETUSER alice on ~* &* +@all nopass`
    - `ACL SETUSER bob on ~* &* +@all >testpass`
  - a Redis instance with `requirepass testpass` active on `localhost:6380`
