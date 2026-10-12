Drupal Redis Stats
==================

[![GoDoc](https://pkg.go.dev/badge/github.com/fgm/drupal_redis_stats)](https://pkg.go.dev/github.com/fgm/drupal_redis_stats)
[![CI](https://github.com/fgm/drupal_redis_stats/actions/workflows/tests.yml/badge.svg)](https://github.com/fgm/drupal_redis_stats/actions/workflows/tests.yml)
[![codecov](https://codecov.io/gh/fgm/drupal_redis_stats/branch/main/graph/badge.svg?token=QR0XKBK3DF)](https://codecov.io/gh/fgm/drupal_redis_stats)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/fgm/drupal_redis_stats/badge)](https://scorecard.dev/viewer/?uri=github.com/fgm/drupal_redis_stats)
[![FOSSA Status](https://app.fossa.com/api/projects/custom%2B11916%2Fgithub.com%2Ffgm%2Fdrupal_redis_stats.svg?type=shield)](https://app.fossa.com/projects/custom%2B11916%2Fgithub.com%2Ffgm%2Fdrupal_redis_stats?ref=badge_shield)

This command summarizes the use of a Redis database
by the Drupal [redis module](https://www.drupal.org/project/redis):
version 1.x for Drupal 10 and 11, or version 2.x, still in alpha, for Drupal 10.3 to 12.

It relies on the Redis `SCAN` command instead of `KEYS`,
so it won't block your site when used on production,
and on `MEMORY USAGE`, which needs Redis 4.0 or later.


## Installing

With Go 1.27.2 or later, or with any Go from 1.21,
which downloads the 1.27.2 toolchain by itself:

```
go install github.com/fgm/drupal_redis_stats@latest
```


## Using
### Flags

- `-h` provides help
- `-dsn` selects a Redis other than the default `redis://localhost:6379/0`
  - `-dsn redis://<host>:<port>/<db>` without authentication
  - `-dsn redis://<password>@<host>:<port>/<db>` for `requirepass` AUTH mode
  - `-dsn redis://<user>:<password>@<host>:<port>/<db>` for ACL AUTH mode
- `-user` and `-pass` override the user and password in the DSN
  - `-pass ""` prompts for the password, without echoing it
- `-json` provides JSON output instead of the default human-readable format
- `-prefix` matches the `cache_prefix` setting of the Drupal site, when it has one
  - `$settings['cache_prefix'] = 'mysite_';` needs `-prefix mysite_`
  - `$settings['cache_prefix']['default'] = 'mysite_';` needs `-prefix mysite_` too
  - per-bin entries like `$settings['cache_prefix']['render']` are documented
    by the redis module, but it never applies them, so they need no flag
  - a `cache_prefix` of `'0'` or `FALSE` is the same as none:
    the module ignores it, so the flag must be omitted
  - without it, the scan covers every key under the module default prefix
    `drupal.redis.<version>.<deployment_identifier>.<hash>`:
    every site in the database, and the keys left by earlier Drupal versions
    or deployments, since both change the prefix.
    Passing one such full prefix restricts it to that site and deployment.
- `-q` disables the progress bar used during the database SCAN loop


### Impact on the site

The scan walks the database in `SCAN` steps of about 1000 keys,
and measures each step's keys in a single pipelined round trip.

- Each step costs Redis about 0.7 ms on Redis 8.10 and 1.9 ms on 7.4,
  split into several reads between which it serves its other clients.
- A million cache entries take about 3 seconds over a same-zone network.

The measurements behind these figures, and the choices they support,
are in [ADR-001](docs/adr/001-pipelining-and-a-fixed-scan-count.md).


### Sample results

```
Bin                |  Keys |     Data
-------------------+-------+---------
bootstrap          |    10 |     6056
config             |  7992 |  4741812
data               | 10483 |  7717813
default            |   183 |   130061
discovery          |   240 |   164028
dynamic_page_cache |  6104 | 11225548
entity             |   785 |   506817
menu               |    28 |    30646
page               |  6125 |  5222326
render             | 12877 | 19965812
-------------------+-------+---------
(cache tags)       |   912 |    83904
(locks)            |     2 |      176
(timestamps)       |    13 |      912
-------------------+-------+---------
Total              | 45760 | 49795911
```

- _Keys_ is the number of entries in a cache bin or category.
- _Data_ is the size used by keys and data in Redis storage,
  per the `MEMORY USAGE` command with `SAMPLES 0`, which measures every field
  of the module's cache entries instead of estimating from a sample:
  before Redis 8.4, sampled sizes can be off by orders of magnitude,
  and change at each Redis restart.
- The rows in parentheses are module keys which belong to no cache bin,
  each omitted when it has no keys:
  - _(cache tags)_: the cache tag invalidation checksums.
    Without this row, the site keeps them elsewhere, usually in its database,
    because the redis module's cache tags service is not enabled;
  - _(flood)_: the flood control counters, such as failed logins;
  - _(locks)_: the locks currently held;
  - _(timestamps)_: the last write timestamps `ChainedFastBackend` uses
    for the bins it serves, and the last delete-all timestamp of each bin.
- _Total_ is the size of all the counted keys,
  but the number of keys in the whole database, matched or not.
- Keys under the prefix which match no known format, or which are deleted
  or expire during the scan, are skipped, and their count is reported on stderr.

## 🤖 AI and Automation Policy

We believe in transparency regarding how this project is built and maintained:

* **AI-Assisted Contributions:** Project maintainers and community contributors may use AI tools as development aids to draft code, generate tests, or summarize pull requests. However, **all functional changes undergo strict human review** prior to being merged. The human author/reviewer remains fully accountable for code security, accuracy, and licensing compliance.
* **Automated Dependencies:** This repository may utilize automated tools like [Dependabot](https://github.com/dependabot) and [Renovate Bot](https://github.com/renovatebot/renovate) for package tracking and dependency bumps. These bots handle backend repository maintenance and do not author core project logic autonomously.
* **Contributor Note:** If you submit code generated with AI assistance, you must ensure that your submission aligns with our open-source license guidelines and that you hold the legal right to certify its provenance.

Releases before 2026-10-01 did not use AI.
