Drupal Redis Stats
==================

[![GoDoc](https://pkg.go.dev/badge/github.com/fgm/drupal_redis_stats)](https://pkg.go.dev/github.com/fgm/drupal_redis_stats)
[![CI](https://github.com/fgm/drupal_redis_stats/actions/workflows/tests.yml/badge.svg)](https://github.com/fgm/drupal_redis_stats/actions/workflows/tests.yml)
[![codecov](https://codecov.io/gh/fgm/drupal_redis_stats/branch/main/graph/badge.svg?token=QR0XKBK3DF)](https://codecov.io/gh/fgm/drupal_redis_stats)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/fgm/drupal_redis_stats/badge)](https://scorecard.dev/viewer/?uri=github.com/fgm/drupal_redis_stats)
[![FOSSA Status](https://app.fossa.com/api/projects/custom%2B11916%2Fgithub.com%2Ffgm%2Fdrupal_redis_stats.svg?type=shield)](https://app.fossa.com/projects/custom%2B11916%2Fgithub.com%2Ffgm%2Fdrupal_redis_stats?ref=badge_shield)

This command provides a summary of the use of a Redis database by the 
Drupal 12, 11, or 10 cache provider.

It relies on Redis `SCAN` operator instead of `KEYS`, so it won't block your
site when used on production.


## Installing

Assuming Go 1.27 or later available:

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
- `-prefix` matches the `cache_prefix` setting of the Drupal site, when it has one
  - `$settings['cache_prefix'] = 'mysite_';` needs `-prefix mysite_`
  - `$settings['cache_prefix']['default'] = 'mysite_';` needs `-prefix mysite_` too
  - per-bin entries like `$settings['cache_prefix']['render']` are documented
    by the redis module, but it never applies them, so they need no flag
  - without it, the scan covers every site using the module default prefix
    `drupal.redis.<version>.<deployment_identifier>.<hash>` in the database:
    passing one such full prefix restricts it to that site
- `-q` disables the progress bar used during the database SCAN loop


### Sample results

```
Bin                     |  Keys |     Data
------------------------+-------+---------
bootstrap               |    10 |     6056
config                  |  7992 |  4741812
data                    | 10483 |  7717813
default                 |   183 |   130061
discovery               |   240 |   164028
dynamic_page_cache      |  6104 | 11225548
entity                  |   785 |   506817
menu                    |    28 |    30646
page                    |  6125 |  5222326
render                  | 12877 | 19965812
------------------------+-------+---------
(last write timestamps) |     3 |      168
------------------------+-------+---------
Total                   | 44835 | 49711087
```

- _Keys_ is the number of entries in a cache bin.
- _Data_ is the size used by keys and data in Redis storage,
  per the `MEMORY USAGE` command.
- _(last write timestamps)_ counts the keys a `ChainedFastBackend` uses
  to track the last write to each bin it serves.
  They belong to no bin, and the row is omitted when there are none.
- _Total_ is the size of all the counted keys,
  but the number of keys in the whole database, matched or not.
- Keys under the prefix which match no known format are skipped,
  and their count is reported on stderr.

## 🤖 AI and Automation Policy

We believe in transparency regarding how this project is built and maintained:

* **AI-Assisted Contributions:** Project maintainers and community contributors may use AI tools as development aids to draft code, generate tests, or summarize pull requests. However, **all functional changes undergo strict human review** prior to being merged. The human author/reviewer remains fully accountable for code security, accuracy, and licensing compliance.
* **Automated Dependencies:** This repository may utilize automated tools like [Dependabot](https://github.com) and [Renovate Bot](https://github.com) for package tracking and dependency bumps. These bots handle backend repository maintenance and do not author core project logic autonomously.
* **Contributor Note:** If you submit code generated with AI assistance, you must ensure that your submission aligns with our open-source license guidelines and that you hold the legal right to certify its provenance.

Releases before 2026-10-01 did not use AI.
