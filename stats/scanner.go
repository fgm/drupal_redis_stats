package stats

import (
	"fmt"
	"io"
	"regexp"

	"github.com/gomodule/redigo/redis"

	redis2 "github.com/fgm/drupal_redis_stats/redis"
	"github.com/fgm/drupal_redis_stats/stats/progress"
)

// NewScanner builds a Scanner
func NewScanner(conn *redis2.AuthConn) Scanner {
	return &RealScanner{Conn: conn}
}

// RealScanner is a concrete Scanner implementation.
type RealScanner struct {
	redis.Conn
	*CacheStats
}

// Scanner describes a service providing statistics for Drupal cache bins in Redis.
type Scanner interface {
	Scan(w io.Writer, maxPasses uint32) (*CacheStats, error)
	io.Closer
}

/*
Scan examines the active database for keys matching the Drupal cache bin format.

  - c is the established connection to Redis on which to perform the Scan.
  - maxPasses allows limiting the number of Redis SCAN steps. Use 0 for no limit.
  - writer is a logging output (think os.Stderr), not the main output.
*/
func (s *RealScanner) Scan(w io.Writer, maxPasses uint32) (*CacheStats, error) {
	s.CacheStats = &CacheStats{
		Stats: map[string]BinStats{},
	}

	dbSize, err := redis.Uint64(s.Do("DBSIZE"))
	if err != nil {
		return nil, err
	}
	s.TotalKeys = uint32(dbSize) // Cannot be >= 2^32 in Redis anyway.
	pb := progress.MakeProgressBar(80, s.TotalKeys)
	var passes uint32 // The number of performed SCAN passes.
	var seen float64
	var iterator int  // Type chosen by Redigo
	var keys []string // The keys returned by a single SCAN pass.
	for {
		passes++
		// Run one Scan pass with the current iterator position.
		arr, err := redis.Values(s.Do("SCAN", iterator, "MATCH", "drupal.redis.*"))
		if err != nil {
			return nil, err
		}
		// Parse Scan pass results.
		iterator, _ = redis.Int(arr[0], nil)
		keys, _ = redis.Strings(arr[1], nil)
		seen += float64(len(keys))
		_, _ = fmt.Fprint(w, pb.Render(seen))
		err = s.indexKeys(keys)
		if err != nil {
			return nil, err
		}
		// When iteration is done, the returned iterator will be 0.
		if iterator == 0 || (maxPasses != 0 && passes >= maxPasses) {
			break
		}
	}
	_, _ = fmt.Fprint(w, pb.Remove())
	return s.CacheStats, nil
}

// indexKeys assumes cs.Stats is already initialized to a non-nil value.
func (s *RealScanner) indexKeys(keys []string) error {
	const reString = `drupal\.redis\.[-.\d\w]+:([\w]+):(.*)`

	var re = regexp.MustCompile(reString)

	for _, key := range keys {
		sl := re.FindStringSubmatch(key)
		if sl == nil {
			return fmt.Errorf("unexpected non-matching key: %s", key)
		}
		bin := sl[1]
		if _, ok := s.Stats[bin]; !ok {
			s.Stats[bin] = BinStats{}
		}
		binStats := s.Stats[bin]
		binStats.addEntry(s.Conn, key)
		s.Stats[bin] = binStats
	}
	return nil
}
