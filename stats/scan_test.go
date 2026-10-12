package stats

import (
	"fmt"
	"io"
	"slices"
	"strconv"
	"testing"
)

// fakeConn is a redis.Conn serving DBSIZE and SCAN through Do,
// and MEMORY USAGE through Send, Flush and Receive only, from fixed data.
//
// SCAN returns one page per call, as Redis would after applying MATCH,
// so the test controls paging and records the MATCH pattern it received.
type fakeConn struct {
	dbSize  int64
	pages   [][]string
	expired map[string]bool // Keys for which MEMORY USAGE answers nil.
	matches []string
	counts  []uint
	flushes int

	sent    [][]any
	replies []any
}

func (fc *fakeConn) Close() error { return nil }
func (fc *fakeConn) Err() error   { return nil }

func (fc *fakeConn) Send(commandName string, args ...any) error {
	fc.sent = append(fc.sent, append([]any{commandName}, args...))
	return nil
}

func (fc *fakeConn) Flush() error {
	fc.flushes++
	for _, command := range fc.sent {
		reply, err := fc.memoryUsage(command[0].(string), command[1:]...)
		if err != nil {
			return err
		}
		fc.replies = append(fc.replies, reply)
	}
	fc.sent = nil
	return nil
}

func (fc *fakeConn) Receive() (reply any, err error) {
	if len(fc.replies) == 0 {
		return nil, fmt.Errorf("Receive without a pending reply")
	}
	reply, fc.replies = fc.replies[0], fc.replies[1:]
	return reply, nil
}

func (fc *fakeConn) memoryUsage(commandName string, args ...any) (any, error) {
	// Without SAMPLES 0, Redis before 8.4 estimates hash sizes from a sample of fields.
	if commandName != "MEMORY" || len(args) != 4 || args[0] != "USAGE" || args[2] != "SAMPLES" || args[3] != 0 {
		return nil, fmt.Errorf("unexpected pipelined command %s %v", commandName, args)
	}
	key := args[1].(string)
	if fc.expired[key] {
		return nil, nil
	}
	// The size is the key length, so each key has a distinct, predictable size.
	return int64(len(key)), nil
}

func (fc *fakeConn) Do(commandName string, args ...any) (any, error) {
	switch commandName {
	case "DBSIZE":
		return fc.dbSize, nil
	case "SCAN":
		page := args[0].(int)
		fc.matches = append(fc.matches, args[2].(string))
		if len(args) != 5 || args[3] != "COUNT" {
			return nil, fmt.Errorf("SCAN without COUNT: %v", args)
		}
		fc.counts = append(fc.counts, args[4].(uint))
		next := page + 1
		if next == len(fc.pages) {
			next = 0
		}
		keys := make([]any, len(fc.pages[page]))
		for i, k := range fc.pages[page] {
			keys[i] = []byte(k)
		}
		return []any{[]byte(strconv.Itoa(next)), keys}, nil
	default:
		return nil, fmt.Errorf("unexpected command %s", commandName)
	}
}

func TestScan(t *testing.T) {
	const p = "mysite_"
	fc := &fakeConn{
		dbSize: 9,
		pages: [][]string{
			{p + ":render:a", p + ":render:bb", p + ":last_write_timestamp_cache_config"},
			{p + ":data:c", p + ":config", p + ":not-a-bin:x", p + ":data:gone"},
			{p + ":render:_redis_last_delete_all", p + ":cachetags:node_list", p + ":flood:user.failed_login_ip:1.2.3.4", p + ":lock:cron"},
			{p + ":not-a-bin:y"},
		},
		expired: map[string]bool{p + ":data:gone": true},
	}
	cs := CacheStats{Prefix: p}
	if err := cs.Scan(fc, 1000, 0, io.Discard); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if expected := []string{p + ":*", p + ":*", p + ":*", p + ":*"}; !slices.Equal(fc.matches, expected) {
		t.Errorf("got MATCH patterns %q, expected %q", fc.matches, expected)
	}
	if expected := []uint{1000, 1000, 1000, 1000}; !slices.Equal(fc.counts, expected) {
		t.Errorf("got COUNT values %v, expected %v", fc.counts, expected)
	}
	// One round trip per batch with recognized keys, none for the last batch.
	if fc.flushes != 3 {
		t.Errorf("got %d flushes, expected 3", fc.flushes)
	}
	if len(fc.replies) != 0 {
		t.Errorf("got %d unread replies, expected none", len(fc.replies))
	}
	if cs.Expired != 1 {
		t.Errorf("got %d expired keys, expected 1", cs.Expired)
	}
	if cs.TotalKeys != 9 {
		t.Errorf("got %d total keys, expected 9", cs.TotalKeys)
	}
	size := func(keys ...string) (sum int64) {
		for _, k := range keys {
			sum += int64(len(k))
		}
		return sum
	}
	expectedBins := map[string]BinStats{
		"render": {Keys: 2, Size: size(p+":render:a", p+":render:bb")},
		"data":   {Keys: 1, Size: size(p + ":data:c")},
	}
	if len(cs.Stats) != len(expectedBins) {
		t.Errorf("got bins %v, expected %v", cs.Stats, expectedBins)
	}
	for bin, expected := range expectedBins {
		if actual := cs.Stats[bin]; actual != expected {
			t.Errorf("bin %s: got %+v, expected %+v", bin, actual, expected)
		}
	}
	others := [...]struct {
		name             string
		actual, expected BinStats
	}{
		{"timestamps", cs.Timestamps, BinStats{Keys: 2, Size: size(p+":last_write_timestamp_cache_config", p+":render:_redis_last_delete_all")}},
		{"cache tags", cs.CacheTags, BinStats{Keys: 1, Size: size(p + ":cachetags:node_list")}},
		{"flood", cs.Flood, BinStats{Keys: 1, Size: size(p + ":flood:user.failed_login_ip:1.2.3.4")}},
		{"locks", cs.Locks, BinStats{Keys: 1, Size: size(p + ":lock:cron")}},
	}
	for _, other := range others {
		if other.actual != other.expected {
			t.Errorf("got %s %+v, expected %+v", other.name, other.actual, other.expected)
		}
	}
	if cs.Unrecognized != 3 {
		t.Errorf("got %d unrecognized keys, expected 3", cs.Unrecognized)
	}
}

func TestScanMaxPasses(t *testing.T) {
	fc := &fakeConn{pages: [][]string{{"drupal.redis.1:render:a"}, {"drupal.redis.1:render:b"}}}
	var cs CacheStats
	if err := cs.Scan(fc, 7, 1, io.Discard); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if expected := []string{"drupal.redis.*"}; !slices.Equal(fc.matches, expected) {
		t.Errorf("got MATCH patterns %q, expected %q", fc.matches, expected)
	}
	if expected := []uint{7}; !slices.Equal(fc.counts, expected) {
		t.Errorf("got COUNT values %v, expected %v", fc.counts, expected)
	}
	if actual := cs.Stats["render"].Keys; actual != 1 {
		t.Errorf("got %d render keys, expected 1", actual)
	}
}

func TestScanZeroCount(t *testing.T) {
	var cs CacheStats
	if err := cs.Scan(&fakeConn{}, 0, 0, io.Discard); err == nil {
		t.Error("expected an error for a zero COUNT")
	}
}
