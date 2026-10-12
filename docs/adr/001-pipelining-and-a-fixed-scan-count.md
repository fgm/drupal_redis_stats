# ADR-001. Pipelining MEMORY USAGE, with a fixed SCAN COUNT

- Status: Accepted
- Date: 2026-10-12
- Ticket: #17
- Scope: how the scan reaches Redis: the batching of the size measurements, and the SCAN COUNT hint.
  `MEMORY USAGE … SAMPLES 0`, also from #17, is a prerequisite:
  its cost is measured below, its reasons are in the code.

## Context

The scan walks the database with `SCAN … MATCH <prefix> COUNT <n>`,
then measures each matching key with `MEMORY USAGE <key> SAMPLES 0`.

Until this decision, every `MEMORY USAGE` was a separate round trip,
sent only after the previous reply arrived.
Sites using Redis for their Drupal cache are usually the large ones,
with a million cache entries or more,
so one round trip per key decides the duration of the whole scan.

`COUNT` is a hint: each `SCAN` step examines about that many keys,
and Redis serves no other client during that step.
A larger value means fewer steps, each blocking the site's Redis longer.

## Decision

1. **Pipeline the `MEMORY USAGE` commands of each `SCAN` batch**:
   queue them all, send them at once, then read the replies in order.
   A batch costs one round trip, instead of one per key.
2. **Use a fixed `COUNT` of 1000**, `stats.ScanCount`, with no command-line flag.

Keys deleted or expired between `SCAN` and `MEMORY USAGE`, for which Redis answers nil,
are skipped and reported on stderr, instead of aborting the scan.

## Measurements

### Over a network: AWS, 1M cache entries

Setup, 2026-10-12, eu-west-3c, spot instances, Amazon Linux 2023 arm64:

| role   | instance                                       | software                                                 |
|--------|------------------------------------------------|----------------------------------------------------------|
| server | r7g.large, 2 vCPU, 16 GiB                      | Docker `redis:7.4.1` then `redis:8.10.2`, no persistence |
| client | t4g.small: then on-demand after a spot reclaim |                                                          |

- Round trip from client to server: 0.19 ms on average (`redis-cli --latency`).
- Data: 1,000,000 hashes shaped like the redis module's cache entries:
  the 8 fields `cid`, `created`, `expire`, `tags`, `valid`, `checksum`, `data`, `serialized`,
  spread over 8 bins.

| `data` size      | 200 B | 1,000 B | 3,000 B | 10,000 B | 40,000 B |
|------------------|-------|---------|---------|----------|----------|
| share of entries | 30%   | 30%     | 20%     | 15%      | 5%       |

| Redis  | memory used | load time, pipelined |
|--------|-------------|----------------------|
| 7.4.1  | 4.81 GB     | 24 s                 |
| 8.10.2 | 4.66 GB     | not measured         |

The spot client was reclaimed during the sequential run on 7.4.1 at `COUNT` 10,000.
That run was repeated from an on-demand client against the same data.

Whole scan, and Redis time per command from `INFO commandstats`,
the chosen `COUNT` of 1000 in bold.
Before this change, the scan ran sequentially with Redis's default `COUNT` of 10:
the first row of each version.

| Redis  | mode       |  `COUNT` |      scan | `SCAN` steps | Redis time per `SCAN` | Redis time per `MEMORY USAGE` |
|--------|------------|---------:|----------:|-------------:|----------------------:|------------------------------:|
| 7.4.1  | sequential |       10 |   189.7 s |       95,436 |              0.009 ms |                  not recorded |
| 7.4.1  | sequential |     1000 |   174.5 s |        1,000 |               0.74 ms |                  not recorded |
| 7.4.1  | sequential |   10,000 |   184.7 s |          100 |               6.98 ms |                       2.16 µs |
| 7.4.1  | pipelined  |       10 |    44.4 s |       95,495 |              0.008 ms |                       1.53 µs |
| 7.4.1  | pipelined  | **1000** | **3.3 s** |        1,000 |           **0.59 ms** |                       1.33 µs |
| 7.4.1  | pipelined  |   10,000 |     3.0 s |          100 |               6.27 ms |                       1.47 µs |
| 8.10.2 | sequential |       10 |   204.4 s |       95,405 |              0.007 ms |                       0.33 µs |
| 8.10.2 | sequential |     1000 |   197.5 s |        1,000 |               0.57 ms |                       0.69 µs |
| 8.10.2 | sequential |   10,000 |   175.7 s |          100 |               5.42 ms |                       0.75 µs |
| 8.10.2 | pipelined  |       10 |    39.6 s |       95,405 |              0.006 ms |                       0.26 µs |
| 8.10.2 | pipelined  | **1000** | **3.0 s** |        1,000 |           **0.45 ms** |                       0.25 µs |
| 8.10.2 | pipelined  |   10,000 |     3.0 s |          100 |               4.89 ms |                       0.24 µs |

- Redis spends at most 2.2 s on the million `MEMORY USAGE` commands:
  the rest of the sequential scans is network round trips.
- Every run reported the same total size per Redis version,
  pipelined or not, and across the restart between the 7.4.1 runs:

| Redis  | total size, all runs |
|--------|---------------------:|
| 7.4.1  |      5,154,641,560 B |
| 8.10.2 |      5,032,644,486 B |

### On loopback: local, 200,000 entries

Setup, 2026-10-12: Apple M1 Max, Redis 8.10.2 from Homebrew on the same machine,
reached over loopback TCP.
Data: 200,000 two-field hashes, `cid` and `data` of 100, 300 or 800 B, over 6 bins,
loaded from the same seed for both modes, each on a fresh Redis:
the step counts differ slightly because each hash table grew differently.

Whole scan, and Redis time per command from `INFO commandstats`,
the chosen `COUNT` of 1000 in bold:

| mode       |  `COUNT` |       scan | `SCAN` steps | Redis time per `SCAN` | Redis time per `MEMORY USAGE` |
|------------|---------:|-----------:|-------------:|----------------------:|------------------------------:|
| sequential |       10 |     29.3 s |       19,265 |              0.005 ms |                  not recorded |
| sequential |     1000 |     26.2 s |          200 |               0.35 ms |                  not recorded |
| sequential |   10,000 |     26.7 s |           20 |                3.1 ms |                  not recorded |
| sequential |  100,000 |     27.1 s |            2 |                 25 ms |                  not recorded |
| pipelined  |       10 |     5.93 s |       19,263 |              0.004 ms |                       0.14 µs |
| pipelined  | **1000** | **0.35 s** |          200 |           **0.30 ms** |                       0.12 µs |
| pipelined  |   10,000 |     0.30 s |           20 |                3.0 ms |                       0.12 µs |
| pipelined  |  100,000 |     0.29 s |            3 |                 17 ms |                       0.12 µs |

- Even on loopback, a round trip costs about 0.13 ms,
  so pipelining divides the scan time by 75 at `COUNT` 1000.
- Sequentially, `COUNT` barely changes the duration, since the per-key round trips dominate.
- Pipelined, it behaves as over the network:
  no gain above 1000, while each step blocks Redis longer.

### Cost of `SAMPLES 0`

Setup, 2026-10-12: Docker `redis:7.4.1` on the same Mac, module-shaped 8-field hashes,
sequential client.

| entries | `data` sizes      | sampled `MEMORY USAGE` |            `SAMPLES 0` |
|--------:|-------------------|-----------------------:|-----------------------:|
|   5,000 | 100 B to 50 KB    | 1.18 to 1.78 s, 3 runs | 1.01 to 1.31 s, 3 runs |
| 200,000 | 100, 300 or 800 B |                 39.7 s |                 38.8 s |

Measuring every field costs nothing measurable, so `SAMPLES 0` needs no flag either.

## Alternatives rejected

### A configurable `COUNT`

From the pipelined runs above:

|  `COUNT` | scan on 7.4.1 | scan on 8.10.2 | Redis time per `SCAN` step |
|---------:|--------------:|---------------:|---------------------------:|
|       10 |        44.4 s |         39.6 s |          0.006 to 0.008 ms |
| **1000** |     **3.3 s** |      **3.0 s** |        **0.45 to 0.59 ms** |
|   10,000 |         3.0 s |          3.0 s |              4.9 to 6.3 ms |

Above 1000, the scan gets no faster while each step blocks Redis ten times longer.
Below it, the scan slows thirteenfold to shorten each `SCAN` by about half a millisecond.
A flag would only serve to choose a worse value,
and would need validation, help text and documentation to explain why.

### A Lua script per batch

A script can run `SCAN`, then `MEMORY USAGE` on each key, in one call:
this was checked on Redis 8.10.

| batch transport    | round trips per batch | round trips per 1M keys at `COUNT` 1000 | at 0.19 ms each |
|--------------------|----------------------:|----------------------------------------:|----------------:|
| pipelined commands |                     2 |                             about 2,000 |    about 0.38 s |
| one Lua script     |                     1 |                             about 1,000 |    about 0.19 s |

- The gain is about 0.2 s per million keys.
- A script blocks Redis for its whole duration,
  while Redis can serve other clients between the chunks of a pipeline.
- It needs the `@scripting` ACL category, which a cache user is often denied,
  and some managed Redis services restrict scripting.

## Consequences

- A scan takes about 3 s per million cache entries on a same-zone network,
  instead of about 3 minutes.
- Each step costs Redis about 0.7 ms on 8.10.2 and 1.9 ms on 7.4.1, in the table below.
- A key deleted or expired mid-scan is reported, instead of making the scan panic.
- `COUNT` can only be changed in the code.
  `Scan` still takes it as a parameter, so a flag can be added back
  if a site ever needs shorter `SCAN` commands.

Redis time per step, from the pipelined `COUNT` 1000 rows above:

| Redis  |  `SCAN` | 1000 `MEMORY USAGE` |   step |
|--------|--------:|--------------------:|-------:|
| 7.4.1  | 0.59 ms |             1.33 ms | 1.9 ms |
| 8.10.2 | 0.45 ms |             0.25 ms | 0.7 ms |

The `SCAN` runs as one block.
The pipelined commands arrive in several reads, between which Redis serves its other clients.
