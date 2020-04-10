/*
Package stats extracts Drupal 8 cache information from a Redis database.
*/
package stats

import (
	"fmt"
	"io"
	"regexp"
	"strconv"

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

/*
addEntry stores the key and data size for a Redis key.
*/
func (bs *BinStats) addEntry(c redis.Conn, key string) {
	bs.Keys++
	val, err := redis.Int64(c.Do("MEMORY", "USAGE", key))
	if err != nil {
		panic(fmt.Errorf("failed MEMORY USAGE: %v", err))
	}
	bs.Size += val
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
	//TODO #2
	//drupalPrefix  string
	TotalKeys uint32 // Redis hardcoded limit.
	Stats     map[string]BinStats
}

// indexKeys assumes cs.Stats is already initialized to a non-nil value.
func (cs *CacheStats) indexKeys(c redis.Conn, keys []string) error {
	const reString = `drupal\.redis\.[-.\d\w]+:([\w]+):(.*)`

	var re = regexp.MustCompile(reString)

	for _, key := range keys {
		sl := re.FindStringSubmatch(key)
		if sl == nil {
			return fmt.Errorf("unexpected non-matching key: %s", key)
		}
		bin := sl[1]
		if _, ok := cs.Stats[bin]; !ok {
			cs.Stats[bin] = BinStats{}
		}
		binStats := cs.Stats[bin]
		binStats.addEntry(c, key)
		cs.Stats[bin] = binStats
	}
	return nil
}

/*
MaxBinNameLength returns the length in runes of the longest bin name.
*/
func (cs CacheStats) MaxBinNameLength() int {
	var max int
	for k := range cs.Stats {
		if len(k) > max {
			max = len(k)
		}
	}

	return max
}

/*
Scan examines the active database for keys matching the Drupal cache bin format.

  - c is the established connection to Redis on which to perform the Scan.
  - maxPasses allows limiting the number of Redis SCAN steps. Use 0 for no limit.
  - writer is a logging output (think os.Stderr), not the main output.
*/
func (cs *CacheStats) Scan(c redis.Conn, maxPasses uint32, w io.Writer) error {
	if cs.Stats == nil {
		cs.Stats = map[string]BinStats{}
	}

	dbSize, err := redis.Uint64(c.Do("DBSIZE"))
	if err != nil {
		return err
	}
	cs.TotalKeys = uint32(dbSize) // Cannot be >= 2^32 in Redis anyway.
	pb := progress.MakeProgressBar(80, cs.TotalKeys)
	var passes uint32 // The number of performed SCAN passes.
	var seen float64
	var iterator int  // Type chosen by Redigo
	var keys []string // The keys returned by a single SCAN pass.
	for {
		passes++
		// Run one Scan pass with the current iterator position.
		arr, err := redis.Values(c.Do("SCAN", iterator, "MATCH", "drupal.redis.*"))
		if err != nil {
			return err
		}
		// Parse Scan pass results.
		iterator, _ = redis.Int(arr[0], nil)
		keys, _ = redis.Strings(arr[1], nil)
		seen += float64(len(keys))
		_, _ = fmt.Fprint(w, pb.Render(seen))
		err = cs.indexKeys(c, keys)
		if err != nil {
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

/*
ItemCountLength returns the length in runes of the total number of keys.

Since this is the sum of all bins, it is at least as large as any per-bin info.
*/
func (cs CacheStats) ItemCountLength() uint {
	return uint(len(strconv.FormatUint(uint64(cs.TotalKeys), 10)))
}

/*
TotalSize returns the total ize used by the cache.

By Redis MEMORY USAGE description, this includes both keys and data.
*/
func (cs CacheStats) TotalSize() int64 {
	var sizeSum int64
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
