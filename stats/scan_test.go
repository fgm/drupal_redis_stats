package stats

import (
	"errors"
	"io"
	"net"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/gomodule/redigo/redis"

	"github.com/fgm/drupal_redis_stats/internal/redistest"
)

func TestMain(m *testing.M) {
	os.Exit(redistest.Main(m))
}

// recordingConn wraps a connection to a real server, recording the SCAN
// arguments and the pipelined commands.
// When set, afterScan runs after each SCAN reply, before its keys are measured.
type recordingConn struct {
	redis.Conn
	matches   []string
	counts    []uint
	sends     int
	flushes   int
	afterScan func()
}

func (rc *recordingConn) Do(commandName string, args ...any) (any, error) {
	if commandName == "SCAN" {
		rc.matches = append(rc.matches, args[2].(string))
		rc.counts = append(rc.counts, args[4].(uint))
	}
	reply, err := rc.Conn.Do(commandName, args...)
	if commandName == "SCAN" && rc.afterScan != nil {
		rc.afterScan()
	}
	return reply, err
}

func (rc *recordingConn) Send(commandName string, args ...any) error {
	rc.sends++
	return rc.Conn.Send(commandName, args...)
}

func (rc *recordingConn) Flush() error {
	rc.flushes++
	return rc.Conn.Flush()
}

// assertUsable fails the test unless Scan read every reply it queued on c,
// and c still answers PING.
//
// A later Do would silently discard the replies left unread, so they are read
// with Do(""), which returns them, instead of being inferred from the PING reply.
func assertUsable(t *testing.T, c redis.Conn) {
	t.Helper()
	left, err := c.Do("")
	if err != nil {
		t.Errorf("failed reading the replies left unread: %v", err)
	} else if left != nil {
		t.Errorf("got %d replies left unread by the scan, expected none", len(left.([]any)))
	}
	if pong, err := redis.String(c.Do("PING")); err != nil || pong != "PONG" {
		t.Errorf("got %q, %v after the scan, expected PONG", pong, err)
	}
}

func TestScan(t *testing.T) {
	for _, s := range redistest.Servers(t) {
		t.Run(s.Image, func(t *testing.T) {
			t.Parallel()
			const p = "mysite_"
			db := s.DB(t)
			setup := s.Dial(t, db)

			// A hash with one large field among many small ones: a sample of
			// its fields either includes the large one, and overestimates it
			// a hundredfold, or misses it, and underestimates it.
			big := []any{p + ":data:big", "data", strings.Repeat("x", 100_000)}
			for i := range 999 {
				big = append(big, "f"+strconv.Itoa(i), "v")
			}
			if _, err := setup.Do("HSET", big...); err != nil {
				t.Fatalf("failed creating %s: %v", big[0], err)
			}
			strs := []string{
				p + ":render:a", p + ":render:bb", p + ":last_write_timestamp_cache_config",
				p + ":data:c", p + ":config", p + ":not-a-bin:x", p + ":data:gone",
				p + ":render:_redis_last_delete_all", p + ":cachetags:node_list",
				p + ":flood:user.failed_login_ip:1.2.3.4", p + ":lock:cron", p + ":not-a-bin:y",
			}
			for _, k := range strs {
				if _, err := setup.Do("SET", k, k); err != nil {
					t.Fatalf("failed creating %s: %v", k, err)
				}
			}

			sizes := map[string]int64{}
			for _, k := range append(strs, p+":data:big") {
				size, err := redis.Int64(setup.Do("MEMORY", "USAGE", k, "SAMPLES", 0))
				if err != nil {
					t.Fatalf("failed measuring %s: %v", k, err)
				}
				sizes[k] = size
			}
			sampled, err := redis.Int64(setup.Do("MEMORY", "USAGE", p+":data:big"))
			if err != nil {
				t.Fatalf("failed measuring %s with the default sample: %v", p+":data:big", err)
			}
			// From Redis 8.4, sampled and exact sizes are equal. Before it, the
			// fixture must make them differ, for the totals below to show that
			// Scan measures with SAMPLES 0.
			if major, minor := s.Version(t); redistest.SamplesHashFields(major, minor) && sampled == sizes[p+":data:big"] {
				t.Errorf("%s sampled %s at its exact size %d: the fixture cannot show the use of SAMPLES 0",
					s.Image, p+":data:big", sampled)
			}

			rc := &recordingConn{Conn: s.Dial(t, db)}
			// Deleted between SCAN and MEMORY USAGE, so Redis answers nil.
			rc.afterScan = func() {
				if _, err := setup.Do("DEL", p+":data:gone"); err != nil {
					t.Errorf("failed deleting %s: %v", p+":data:gone", err)
				}
			}
			cs := CacheStats{Prefix: p}
			if err := cs.Scan(rc, ScanCount, 0, io.Discard); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertUsable(t, rc)

			for _, match := range rc.matches {
				if match != p+":*" {
					t.Errorf("got MATCH pattern %q, expected %q", match, p+":*")
				}
			}
			for _, count := range rc.counts {
				if count != ScanCount {
					t.Errorf("got COUNT %d, expected %d", count, ScanCount)
				}
			}
			// One MEMORY USAGE per key of a known format, at most one round trip per SCAN step.
			if rc.sends != 10 {
				t.Errorf("got %d MEMORY USAGE commands, expected 10", rc.sends)
			}
			if rc.flushes == 0 || rc.flushes > len(rc.matches) {
				t.Errorf("got %d flushes for %d SCAN steps, expected 1 to %d", rc.flushes, len(rc.matches), len(rc.matches))
			}
			if cs.Vanished != 1 {
				t.Errorf("got %d vanished keys, expected 1", cs.Vanished)
			}
			if cs.TotalKeys != 13 {
				t.Errorf("got %d total keys, expected 13", cs.TotalKeys)
			}
			size := func(keys ...string) (sum int64) {
				for _, k := range keys {
					sum += sizes[k]
				}
				return sum
			}
			expectedBins := map[string]BinStats{
				"render": {Keys: 2, Size: size(p+":render:a", p+":render:bb")},
				"data":   {Keys: 2, Size: size(p+":data:c", p+":data:big")},
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
		})
	}
}

func TestScanMaxPasses(t *testing.T) {
	const keys = 50
	for _, s := range redistest.Servers(t) {
		t.Run(s.Image, func(t *testing.T) {
			t.Parallel()
			db := s.DB(t)
			c := s.Dial(t, db)
			for i := range keys {
				if _, err := c.Do("SET", "drupal.redis.1:render:"+strconv.Itoa(i), "v"); err != nil {
					t.Fatalf("failed creating key %d: %v", i, err)
				}
			}
			rc := &recordingConn{Conn: c}
			var cs CacheStats
			if err := cs.Scan(rc, 7, 1, io.Discard); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if expected := []string{"drupal.redis.*"}; !slices.Equal(rc.matches, expected) {
				t.Errorf("got MATCH patterns %q, expected %q", rc.matches, expected)
			}
			if expected := []uint{7}; !slices.Equal(rc.counts, expected) {
				t.Errorf("got COUNT values %v, expected %v", rc.counts, expected)
			}
			if actual := cs.Stats["render"].Keys; actual == 0 || actual >= keys {
				t.Errorf("got %d render keys after one SCAN step, expected 1 to %d", actual, keys-1)
			}
		})
	}
}

func TestScanOnlyUnrecognized(t *testing.T) {
	for _, s := range redistest.Servers(t) {
		t.Run(s.Image, func(t *testing.T) {
			t.Parallel()
			db := s.DB(t)
			c := s.Dial(t, db)
			if _, err := c.Do("SET", "drupal.redis.1:no_bin", "v"); err != nil {
				t.Fatalf("failed creating key: %v", err)
			}
			rc := &recordingConn{Conn: c}
			var cs CacheStats
			if err := cs.Scan(rc, ScanCount, 0, io.Discard); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			// A batch with nothing to measure costs no round trip.
			if rc.sends != 0 || rc.flushes != 0 {
				t.Errorf("got %d sends and %d flushes, expected none", rc.sends, rc.flushes)
			}
			if cs.Unrecognized != 1 {
				t.Errorf("got %d unrecognized keys, expected 1", cs.Unrecognized)
			}
		})
	}
}

func TestScanZeroCount(t *testing.T) {
	var cs CacheStats
	// The count is checked before any use of the connection.
	if err := cs.Scan(nil, 0, 0, io.Discard); err == nil {
		t.Error("expected an error for a zero COUNT")
	}
}

func TestScanErrorReply(t *testing.T) {
	const p = "drupal.redis.1"
	for _, s := range redistest.Servers(t) {
		t.Run(s.Image, func(t *testing.T) {
			t.Parallel()
			db := s.DB(t)
			setup := s.Dial(t, db)
			for _, k := range []string{p + ":render:a", p + ":render:b", p + ":render:c"} {
				if _, err := setup.Do("SET", k, k); err != nil {
					t.Fatalf("failed creating %s: %v", k, err)
				}
			}
			user, pass := s.User(t, "~*", "+@all", "-memory|usage")
			c, err := redis.DialURL(s.URL(user, pass, db))
			if err != nil {
				t.Fatalf("failed dialing as %s: %v", user, err)
			}
			defer c.Close()

			var cs CacheStats
			err = cs.Scan(c, ScanCount, 0, io.Discard)
			var reply redis.Error
			if !errors.As(err, &reply) || !strings.HasPrefix(string(reply), "NOPERM") {
				t.Fatalf("got error %v, expected a NOPERM error reply", err)
			}
			// An error reply leaves the connection usable: no reply may be left unread.
			assertUsable(t, c)
			if cs.Vanished != 0 {
				t.Errorf("got %d vanished keys, expected none: an error reply is not a missing key", cs.Vanished)
			}
		})
	}
}

func TestScanTransportErrors(t *testing.T) {
	// Each pipelined command is buffered, and Send only writes when the buffer
	// is full, so a large batch fails on Send, and a small one on Flush.
	// A socket closed before the scan fails its first command, DBSIZE.
	tests := [...]struct {
		name      string
		keys      int
		closeLate bool
		expected  string
	}{
		{"dbsize", 1, false, ""},
		{"send", 1000, true, "failed queuing MEMORY USAGE"},
		{"flush", 1, true, "failed sending MEMORY USAGE"},
	}
	for _, s := range redistest.Servers(t) {
		for _, test := range tests {
			t.Run(s.Image+"/"+test.name, func(t *testing.T) {
				t.Parallel()
				db := s.DB(t)
				setup := s.Dial(t, db)
				for i := range test.keys {
					if _, err := setup.Do("SET", "drupal.redis.1:render:"+strconv.Itoa(i), "v"); err != nil {
						t.Fatalf("failed creating key %d: %v", i, err)
					}
				}
				var netConn net.Conn
				c := s.Dial(t, db, redis.DialNetDial(func(network, address string) (net.Conn, error) {
					var err error
					netConn, err = net.Dial(network, address)
					return netConn, err
				}))
				// Closing the socket between SCAN and MEMORY USAGE breaks the connection.
				rc := &recordingConn{Conn: c, afterScan: func() { _ = netConn.Close() }}
				if !test.closeLate {
					_ = netConn.Close()
				}

				var cs CacheStats
				err := cs.Scan(rc, ScanCount, 0, io.Discard)
				if !errors.Is(err, net.ErrClosed) || !strings.HasPrefix(err.Error(), test.expected) {
					t.Errorf("got error %v, expected %q wrapping %v", err, test.expected, net.ErrClosed)
				}
			})
		}
	}
}
