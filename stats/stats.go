/*
Package stats extracts Drupal 8 cache information from a Redis database.
*/
package stats

import (
	"fmt"
	"strconv"

	"github.com/gomodule/redigo/redis"
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
