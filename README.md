Drupal Redis Stats
==================

[![Maintainability](https://api.codeclimate.com/v1/badges/270be4d8262469d3949c/maintainability)](https://codeclimate.com/github/fgm/drupal_redis_stats/maintainability)
[![FOSSA Status](https://app.fossa.com/api/projects/custom%2B11916%2Fgithub.com%2Ffgm%2Fdrupal_redis_stats.svg?type=shield)](https://app.fossa.com/projects/custom%2B11916%2Fgithub.com%2Ffgm%2Fdrupal_redis_stats?ref=badge_shield)

This command provides a summary of the use of a Redis database by the 
Drupal 8 cache provider.

It relies on Redis `SCAN` operator.

## Installing

Assuming Go 1.14 or later installed:

```
go get -u github.com/fgm/drupal_redis_stats
```


## Using
### Flags

- `-h` provides help
- `-dsn redis://<host>:<port>` flag allows using a non-default Redis
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
