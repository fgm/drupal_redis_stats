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

var index = make(map[string]int)

func main() {
	// Connect to the server (ex: DB #1, default #0).
	const dsn = "redis://localhost/"
	c, err := redis.DialURL(dsn, redis.DialDatabase(0))
	if err != nil {
		panic(err)
	}
	defer c.Close()

	// Find all data: https://redis.io/commands/scan
	passes := 0
	iterator := 0
	var keys []string
	var total int
	for {
		passes++
		// Scan with the current iterator position.
		arr, err := redis.Values(c.Do("SCAN", iterator, "MATCH", "drupal.redis.*"))
		if err != nil {
			panic(err)
		}
		iterator, _ = redis.Int(arr[0], nil)
		keys, _ = redis.Strings(arr[1], nil)
		fmt.Fprintf(os.Stderr, "%5d: %7d | ", passes, iterator)
		if passes%10 == 0 {
			fmt.Fprintln(os.Stderr)
		}
		total += len(keys)
		indexKeys(keys)
		// When iteration is done, the returned iterator will be 0.
		if iterator == 0 {
			break
		}
		// Read single value: https://redis.io/commands/get
		//res, err := redis.String(c.Do("GET", strconv.Itoa(k)))
	}
	printStats(os.Stdout, total)
}

func indexKeys(keys []string) {
	for _, key := range keys {
		sl := re.FindAllStringSubmatch(key, -1)
		if sl == nil {
			panic(fmt.Errorf("Unexpected non-matching key %s", key))
		}
		sm := sl[0]
		bin := sm[1]
		index[bin]++
	}
}

func printStats(w io.Writer, total int) {
	bins := []string{}
	binMax := 0
	countMax := 0.0
	for bin, _ := range index {
		bins = append(bins, bin)
		width := math.Ceil(math.Log10(float64(index[bin])))
		if width > countMax {
			countMax = width
		}
		if len(bin) > binMax {
			binMax = len(bin)
		}
	}
	intCountMax := len("Entries")
	if int(countMax) > intCountMax {
		intCountMax = int(countMax)
	}
	format := fmt.Sprintf("%%-%ds | %%%ds\n", binMax, intCountMax)
	sort.Strings(bins)

	fmt.Fprintf(w, format, "Bin", "Entries")
	fmt.Fprintln(w, strings.Repeat("-", binMax) + "-+-" + strings.Repeat("-", intCountMax))
	for _, bin := range bins {
		fmt.Fprintf(w, format, bin, strconv.Itoa(index[bin]))
	}
	fmt.Fprintln(w, strings.Repeat("-", binMax) + "-+-" + strings.Repeat("-", intCountMax))
	fmt.Fprintf(w, format, "Total", strconv.Itoa(total))
}
