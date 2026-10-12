/*
Package redistest runs real Redis servers in containers, shared by the tests of a package.

The package TestMain calls Main, which terminates the servers once the tests are done.
The servers start on the first call to Servers, so a run selecting no test using them starts none.

Each test then reserves its own database with Server.DB,
and creates its own ACL users with Server.User,
so tests can run in parallel on the same servers.
Tests must not use commands acting on the whole server, such as FLUSHALL or CONFIG SET.
*/
package redistest

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/testcontainers/testcontainers-go"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

// Images are the Redis versions the tests run against: the last 7.x release,
// sampling hash fields in MEMORY USAGE by default like every release before 8.4,
// and the current one. TestImagesIncludeSampling requires a sampling one.
var Images = [...]string{"redis:7.4.11", "redis:8.10.2"}

// Password is the requirepass of every server: the password of its default user.
const Password = "redistest"

// startTimeout bounds the start of the servers, image pulls included,
// so a hung Docker daemon fails the tests instead of blocking them
// until the go test timeout. Arbitrary.
const startTimeout = 2 * time.Minute

// databases bounds the databases reserved in a test process, one per call to Server.DB. Arbitrary.
//
// Databases are never released: with go test -count=N, the same servers serve
// all N runs, so N times the reservations of one run must stay below databases.
const databases = 64

// Server is a Redis server in a container.
type Server struct {
	Image    string
	addr     string
	lastDB   atomic.Int32
	lastUser atomic.Int32
}

var (
	once       sync.Once
	servers    []*Server
	containers []*tcredis.RedisContainer
	startErr   error
)

// Main runs the tests of a package, then terminates the servers they started,
// and returns the exit code for os.Exit.
//
// Termination cannot be deferred, since os.Exit skips deferred calls.
// If the process dies first, the Testcontainers reaper, Ryuk, removes the containers.
func Main(m *testing.M) int {
	code := m.Run()
	for _, c := range containers {
		if err := testcontainers.TerminateContainer(c); err != nil {
			log.Printf("failed terminating Redis container: %v", err)
		}
	}
	return code
}

// Servers returns one server per image in Images, starting them on the first call.
//
// When they cannot be started, for instance without Docker, it fails the test:
// a skip would let a run pass without testing anything against Redis.
func Servers(t testing.TB) []*Server {
	t.Helper()
	once.Do(start)
	if startErr != nil {
		t.Fatalf("failed starting Redis containers: %v", startErr)
	}
	return servers
}

// start runs the servers.
//
// Cancelling its context once they run is safe: testcontainers uses it to
// create the containers and to dial its reaper, and a connection outlives
// the context it was dialed with.
func start() {
	ctx, cancel := context.WithTimeout(context.Background(), startTimeout)
	defer cancel()
	results := make([]*tcredis.RedisContainer, len(Images))
	errs := make([]error, len(Images))
	var wg sync.WaitGroup
	for i, image := range Images {
		wg.Go(func() {
			results[i], errs[i] = tcredis.Run(ctx, image, testcontainers.WithCmd("redis-server",
				"--requirepass", Password,
				"--databases", strconv.Itoa(databases),
				"--save", "",
				"--appendonly", "no",
			))
		})
	}
	wg.Wait()
	for i, c := range results {
		if c != nil {
			containers = append(containers, c)
		}
		if errs[i] != nil {
			startErr = errors.Join(startErr, fmt.Errorf("%s: %w", Images[i], errs[i]))
			continue
		}
		addr, err := c.Endpoint(ctx, "")
		if err != nil {
			startErr = errors.Join(startErr, fmt.Errorf("%s: %w", Images[i], err))
			continue
		}
		servers = append(servers, &Server{Image: Images[i], addr: addr})
	}
}

// URL returns the URL of a database on the server, for user and pass.
// An empty user means the default user.
func (s *Server) URL(user, pass string, db int) string {
	return fmt.Sprintf("redis://%s:%s@%s/%d", user, pass, s.addr, db)
}

// DB reserves a database for the test, and returns its number.
//
// Database 0 is never reserved: tests may dial it, but must not write to it.
func (s *Server) DB(t testing.TB) int {
	t.Helper()
	db := int(s.lastDB.Add(1))
	if db >= databases {
		t.Fatalf("all %d databases of %s are reserved, by too many tests or a high -count: see databases in internal/redistest", databases-1, s.Image)
	}
	return db
}

// Dial connects to a database on the server as its default user,
// and closes the connection when the test ends.
func (s *Server) Dial(t testing.TB, db int, options ...redis.DialOption) redis.Conn {
	t.Helper()
	c, err := redis.DialURL(s.URL("", Password, db), options...)
	if err != nil {
		t.Fatalf("failed dialing %s: %v", s.Image, err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// User creates an ACL user for the test, enabled with a password and the given rules,
// returns its name and password, and deletes it when the test ends.
func (s *Server) User(t testing.TB, rules ...string) (name, pass string) {
	t.Helper()
	name = "user" + strconv.Itoa(int(s.lastUser.Add(1)))
	pass = name + "-pass"
	c := s.Dial(t, 0)
	args := []any{"SETUSER", name, "reset", "on", ">" + pass}
	for _, rule := range rules {
		args = append(args, rule)
	}
	if _, err := c.Do("ACL", args...); err != nil {
		t.Fatalf("failed creating ACL user on %s: %v", s.Image, err)
	}
	// Cleanups run last in, first out: this one runs before Dial closes c.
	t.Cleanup(func() { _, _ = c.Do("ACL", "DELUSER", name) })
	return name, pass
}

// Version returns the major and minor version of the server.
func (s *Server) Version(t testing.TB) (major, minor int) {
	t.Helper()
	info, err := redis.String(s.Dial(t, 0).Do("INFO", "server"))
	if err != nil {
		t.Fatalf("failed reading the version of %s: %v", s.Image, err)
	}
	for line := range strings.Lines(info) {
		if v, ok := strings.CutPrefix(line, "redis_version:"); ok {
			if major, minor, err = parseVersion(strings.TrimSpace(v)); err != nil {
				t.Fatalf("failed parsing the version of %s: %v", s.Image, err)
			}
			return major, minor
		}
	}
	t.Fatalf("no redis_version in the INFO of %s", s.Image)
	return 0, 0
}

// parseVersion returns the major and minor numbers of a version such as 7.4.11.
func parseVersion(version string) (major, minor int, err error) {
	parts := strings.SplitN(version, ".", 3)
	if len(parts) < 2 {
		return 0, 0, fmt.Errorf("version %q has no minor number", version)
	}
	if major, err = strconv.Atoi(parts[0]); err != nil {
		return 0, 0, fmt.Errorf("version %q: %w", version, err)
	}
	if minor, err = strconv.Atoi(parts[1]); err != nil {
		return 0, 0, fmt.Errorf("version %q: %w", version, err)
	}
	return major, minor, nil
}

// SamplesHashFields reports whether a Redis version measures the MEMORY USAGE
// of a hash from a sample of its fields by default, as every release before 8.4 does.
func SamplesHashFields(major, minor int) bool {
	return major < 8 || major == 8 && minor < 4
}
