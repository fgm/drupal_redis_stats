package main

import (
	"fmt"
	"io"
	"math"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/gomodule/redigo/redis"
)

const reString = `drupal\.redis\.[.\d\w]+:([\w]+):(.*)`

var re = regexp.MustCompile(reString)

/*
BinStats holds stats for a single Drupal cache bin.
*/
type BinStats struct {
	Keys uint // Redis only supports 2^32 keys anyway.
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

type CacheStats struct {
	memoryUsed    uint64
	memoryPeak    uint64
	drupalVersion string
	drupalPrefix  string
	itemCount     uint32 // Redis hardcoded limit.
	stats         map[string]BinStats
}

func main() {
	// Connect to the server (ex: DB #1, default #0).
	const dsn = "redis://localhost/"
	c, err := redis.DialURL(dsn, redis.DialDatabase(0))
	if err != nil {
		panic(err)
	}
	defer c.Close()

	var stats CacheStats
	if err = stats.Scan(c, 0, os.Stderr); err != nil {
		panic(err)
	}
	printStats(os.Stdout, &stats)
}

/*
Scan examines the active database for keys matching the Drupal cache bin format.

  - c is the established connection to Redis on which to perform the Scan.
  - maxPasses allows limiting the number of Redis SCAN steps. Use 0 for no limit.
  - writer is a logging output (think os.Stderr), not the main output.
*/
func (cs *CacheStats) Scan(c redis.Conn, maxPasses uint32, w io.Writer) error {
	if cs.stats == nil {
		cs.stats = map[string]BinStats{}
	}

	dbSize, err := redis.Uint64(c.Do("DBSIZE"))
	if err != nil {
		return err
	}
	cs.itemCount = uint32(dbSize) // Cannot be >= 2^32 in Redis anyway.
	pb := makeProgressBar(80, cs.itemCount)
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

// indexKeys assumes cs.stats is already initialized to a non-nil value.
func (cs *CacheStats) indexKeys(c redis.Conn, keys []string) error {
	for _, key := range keys {
		sl := re.FindStringSubmatch(key)
		if sl == nil {
			return fmt.Errorf("unexpected non-matching key: %s", key)
		}
		bin := sl[1]
		if _, ok := cs.stats[bin]; !ok {
			cs.stats[bin] = BinStats{}
		}
		binStats := cs.stats[bin]
		binStats.addEntry(c, key)
		cs.stats[bin] = binStats
	}
	return nil
}

func printStats(w io.Writer, cs *CacheStats) {
	var bins []string
	binMax := 0
	countMax := 0.0
	sizeMax := 0.0
	var totalCount uint
	var totalSize int64
	for bin := range cs.stats {
		bins = append(bins, bin)
		keysWidth := math.Ceil(math.Log10(float64(cs.stats[bin].Keys)))
		sizeWidth := math.Ceil(math.Log10(float64(cs.stats[bin].Size)))

		if keysWidth > countMax {
			countMax = keysWidth
		}
		if sizeWidth > sizeMax {
			sizeMax = sizeWidth
		}
		if len(bin) > binMax {
			binMax = len(bin)
		}
	}
	intCountMax := len("Entries")
	if int(countMax) > intCountMax {
		intCountMax = int(countMax)
	}
	intSizeMax := len("Size")
	if int(sizeMax) > intSizeMax {
		intSizeMax = int(sizeMax)
	}
	format := fmt.Sprintf("%%-%ds | %%%ds | %%%ds\n", binMax, intCountMax, intSizeMax)
	sort.Strings(bins)

	_, _ = fmt.Fprintf(w, format, "Bin", "Entries", "Size")
	hr := fmt.Sprint(strings.Repeat("-", binMax) +
		"-+-" + strings.Repeat("-", intCountMax) +
		"-+-" + strings.Repeat("-", intSizeMax))
	_, _ = fmt.Fprintln(w, hr)
	for _, bin := range bins {
		totalCount += cs.stats[bin].Keys
		totalSize += cs.stats[bin].Size
		_, _ = fmt.Fprintf(w, format, bin,
			strconv.FormatUint(uint64(cs.stats[bin].Keys), 10),
			strconv.FormatUint(uint64(cs.stats[bin].Size), 10))
	}
	_, _ = fmt.Fprintln(w, hr)
	_, _ = fmt.Fprintf(w, format, "Total",
		strconv.FormatUint(uint64(totalCount), 10),
		strconv.FormatUint(uint64(totalSize), 10),
	)
}
