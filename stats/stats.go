/*
Package stats extracts Drupal cache information from a Redis database.
*/
package stats

import (
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gomodule/redigo/redis"

	"github.com/fgm/drupal_redis_stats/stats/progress"
)

/*
BinStats holds Stats for a single Drupal cache bin.
*/
type BinStats struct {
	Keys uint32 // Redis only supports 2^32 keys anyway.
	Size int64
}

// add counts one key of the given size.
func (bs *BinStats) add(size int64) {
	bs.Keys++
	bs.Size += size
}

/*
CacheStats represents the discovered information about Drupal cache data held in
a Redis database.
*/
type CacheStats struct {
	//TODO #1 implement these fields.
	//memoryUsed    uint64
	//memoryPeak    uint64
	//drupalVersion string

	// Prefix is the Drupal cache_prefix setting: the string form, or its
	// 'default' entry. Empty means the redis module default prefix.
	Prefix    string `json:",omitempty"`
	TotalKeys uint32 // Redis hardcoded limit.
	Stats     map[string]BinStats

	// CacheTags, Flood and Locks hold the keys of the redis module cache tag
	// checksums, flood control and lock backends, which are not cache bins.
	CacheTags BinStats `json:",omitzero"`
	Flood     BinStats `json:",omitzero"`
	Locks     BinStats `json:",omitzero"`

	// Timestamps holds the last write timestamps of the bins used behind a
	// ChainedFastBackend, and the last delete-all timestamps of each bin.
	// They are not cache entries, so they belong to no bin.
	Timestamps BinStats `json:",omitzero"`

	// Unrecognized counts the keys under the prefix which match no known format.
	Unrecognized uint32 `json:"-"`

	// Expired counts the keys which disappeared between SCAN and MEMORY USAGE.
	Expired uint32 `json:"-"`
}

// globEscape escapes the characters which are special in Redis glob-style patterns.
func globEscape(s string) string {
	return strings.NewReplacer(
		`\`, `\\`, `*`, `\*`, `?`, `\?`, `[`, `\[`, `]`, `\]`,
	).Replace(s)
}

/*
keyPattern returns the SCAN MATCH pattern selecting the Drupal keys for a prefix,
and the regexp classifying those keys.

The redis module builds cache keys as <prefix>:<bin>:<cid>, except for:
  - the last write timestamps of a ChainedFastBackend: <prefix>:last_write_timestamp_<bin>
  - the last delete-all timestamp of a bin: <prefix>:<bin>:_redis_last_delete_all
  - cache tag checksums, flood control and locks: <prefix>:{cachetags,flood,lock}:<name>

When no prefix is configured, it uses
drupal.redis.<version>.<deployment_identifier>.<hash>.
Per-bin cache_prefix entries do not apply: the module never passes a bin
when resolving its prefix, although its README documents them.
*/
func keyPattern(prefix string) (string, *regexp.Regexp) {
	match, rePrefix := "drupal.redis.*", `drupal\.redis\.[-.\d\w]+`
	if prefix != "" {
		match, rePrefix = globEscape(prefix)+":*", regexp.QuoteMeta(prefix)
	}
	return match, regexp.MustCompile(`^` + rePrefix + `:(?:last_write_timestamp_(\w+)$|(cachetags|flood|lock):|(\w+):(_redis_last_delete_all$)?)`)
}

type keyKind int

const (
	unknownKey keyKind = iota
	binKey
	timestampKey
	cacheTagKey
	floodKey
	lockKey
)

// otherKinds maps the non-bin key segments to their kind.
var otherKinds = map[string]keyKind{"cachetags": cacheTagKey, "flood": floodKey, "lock": lockKey}

// classify returns the kind of key, and the name of the bin it relates to, if any.
func classify(re *regexp.Regexp, key string) (keyKind, string) {
	sl := re.FindStringSubmatch(key)
	switch {
	case sl == nil:
		return unknownKey, ""
	case sl[1] != "":
		return timestampKey, sl[1]
	case sl[2] != "":
		return otherKinds[sl[2]], ""
	case sl[4] != "":
		return timestampKey, sl[3]
	default:
		return binKey, sl[3]
	}
}

// add counts a key of the given kind and size, in its bin for a bin key.
func (cs *CacheStats) add(kind keyKind, bin string, size int64) {
	switch kind {
	case binKey:
		binStats := cs.Stats[bin]
		binStats.add(size)
		cs.Stats[bin] = binStats
	case timestampKey:
		cs.Timestamps.add(size)
	case cacheTagKey:
		cs.CacheTags.add(size)
	case floodKey:
		cs.Flood.add(size)
	case lockKey:
		cs.Locks.add(size)
	}
}

/*
indexKeys measures the keys of one SCAN batch, and counts them.

It pipelines the MEMORY USAGE commands, so a batch costs one round trip
instead of one per key, which dominated the scan time over a network.
It assumes cs.Stats is already initialized to a non-nil value.
*/
func (cs *CacheStats) indexKeys(c redis.Conn, re *regexp.Regexp, keys []string) error {
	type pending struct {
		kind keyKind
		bin  string
	}
	queued := make([]pending, 0, len(keys))
	for _, key := range keys {
		kind, bin := classify(re, key)
		if kind == unknownKey {
			cs.Unrecognized++
			continue
		}
		// SAMPLES 0 measures every field of a hash. With the default sample of
		// 5 fields, Redis before 8.4 picks the fields by a hash seed drawn at each
		// process start, so the module's 8-field cache entries measured 0.009
		// times their size in one process and 1.6 times in another: sampled sizes
		// are neither accurate nor reproducible. From Redis 8.4, both forms give
		// the exact size.
		if err := c.Send("MEMORY", "USAGE", key, "SAMPLES", 0); err != nil {
			return fmt.Errorf("failed queuing MEMORY USAGE: %w", err)
		}
		queued = append(queued, pending{kind, bin})
	}
	if len(queued) == 0 {
		return nil
	}
	if err := c.Flush(); err != nil {
		return fmt.Errorf("failed sending MEMORY USAGE: %w", err)
	}
	for _, p := range queued {
		size, err := redis.Int64(c.Receive())
		switch {
		case errors.Is(err, redis.ErrNil):
			cs.Expired++
		case err != nil:
			return fmt.Errorf("failed MEMORY USAGE: %w", err)
		default:
			cs.add(p.kind, p.bin, size)
		}
	}
	return nil
}

/*
MaxBinNameLength returns the length in runes of the longest bin name.
*/
func (cs CacheStats) MaxBinNameLength() int {
	var longest int
	for k := range cs.Stats {
		longest = max(longest, utf8.RuneCountInString(k))
	}
	return longest
}

// ScanCount is the SCAN COUNT hint Scan uses in production:
// see docs/adr/001-pipelining-and-a-fixed-scan-count.md for its measurements.
const ScanCount = 1000

/*
Scan examines the active database for keys matching the Drupal cache bin format,
using cs.Prefix as the Drupal cache_prefix.

  - c is the established connection to Redis on which to perform the Scan.
  - count is the SCAN COUNT hint: the number of buckets each SCAN step examines.
    Redis rejects values below 1.
  - maxPasses allows limiting the number of Redis SCAN steps. Use 0 for no limit.
  - writer is a logging output (think os.Stderr), not the main output.
*/
func (cs *CacheStats) Scan(c redis.Conn, count uint, maxPasses uint32, w io.Writer) error {
	if count == 0 {
		return errors.New("SCAN COUNT must be at least 1")
	}
	if cs.Stats == nil {
		cs.Stats = map[string]BinStats{}
	}

	dbSize, err := redis.Uint64(c.Do("DBSIZE"))
	if err != nil {
		return err
	}
	cs.TotalKeys = uint32(dbSize) // Cannot be >= 2^32 in Redis anyway.
	match, re := keyPattern(cs.Prefix)
	pb := progress.MakeProgressBar(80, cs.TotalKeys)
	var passes uint32 // The number of performed SCAN passes.
	var seen float64
	var iterator int  // Type chosen by Redigo
	var keys []string // The keys returned by a single SCAN pass.
	for {
		passes++
		// Run one Scan pass with the current iterator position.
		arr, err := redis.Values(c.Do("SCAN", iterator, "MATCH", match, "COUNT", count))
		if err != nil {
			return err
		}
		// Parse Scan pass results.
		iterator, _ = redis.Int(arr[0], nil)
		keys, _ = redis.Strings(arr[1], nil)
		seen += float64(len(keys))
		_, _ = fmt.Fprint(w, pb.Render(seen))
		if err = cs.indexKeys(c, re, keys); err != nil {
			return err
		}
		// When iteration is done, the returned iterator will be 0.
		if iterator == 0 || (maxPasses != 0 && passes >= maxPasses) {
			break
		}
	}
	_, _ = fmt.Fprint(w, pb.Remove())
	return nil
}

// MatchPattern returns the SCAN MATCH pattern selecting the keys under cs.Prefix.
func (cs CacheStats) MatchPattern() string {
	match, _ := keyPattern(cs.Prefix)
	return match
}

/*
MatchedKeys returns the number of keys counted in bins or in other categories.
*/
func (cs CacheStats) MatchedKeys() uint32 {
	sum := cs.CacheTags.Keys + cs.Flood.Keys + cs.Locks.Keys + cs.Timestamps.Keys
	for _, v := range cs.Stats {
		sum += v.Keys
	}
	return sum
}

/*
ItemCountLength returns the length in runes of the total number of keys.

Since this is the sum of all bins, it is at least as large as any per-bin info.
*/
func (cs CacheStats) ItemCountLength() uint {
	return uint(len(strconv.FormatUint(uint64(cs.TotalKeys), 10)))
}

/*
TotalSize returns the total size used by the cache.

By Redis MEMORY USAGE description, this includes both keys and data.
*/
func (cs CacheStats) TotalSize() int64 {
	sizeSum := cs.CacheTags.Size + cs.Flood.Size + cs.Locks.Size + cs.Timestamps.Size
	for _, v := range cs.Stats {
		sizeSum += v.Size
	}
	return sizeSum
}

/*
TotalSizeLength returns the length in runes of the cache size, expressed in bytes.

Since this is the sum of all bins, it is at least as large as any per-bin info.
*/
func (cs CacheStats) TotalSizeLength() int {
	return len(strconv.FormatInt(cs.TotalSize(), 10))
}
