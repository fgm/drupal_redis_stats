package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/fgm/drupal_redis_stats/internal/redistest"
	"github.com/gomodule/redigo/redis"
	"golang.org/x/term"
)

const testPassword = "test-password"

func readTestPassword(pass string) passwordReaderFunc {
	return func(_ int) ([]byte, error) {
		return []byte(pass), nil
	}
}

func TestGetPasswordFromCLI(t *testing.T) {
	var err error
	var f *os.File
	checks := [...]struct {
		name     string
		reader   PasswordReader
		input    io.Reader
		expValue string
		expError error
	}{
		{"test happy", readTestPassword(testPassword), f, testPassword, nil},
		{"test sad", readTestPassword(string([]byte{0xC0})), f, testPassword, ErrInvalidUTF8},
		{"real", passwordReaderFunc(term.ReadPassword), os.Stdin, "", syscall.ENOTTY},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			f, err = os.CreateTemp("", "getPasswordFromCLI")
			defer os.Remove(f.Name())
			if err != nil {
				t.Fatalf("failed creating alternate input: %v", err)
			}

			w := strings.Builder{}
			actual, err := getPasswordFromCLI(&w, check.reader, int(f.Fd()))
			if prompt := w.String(); prompt != "Password: \n" {
				t.Errorf("expected prompt %q, got %q", "Password: \n", prompt)
			}
			if check.expError != nil {
				if err == nil {
					t.Fatalf("did not get expected error")
				}
				if !errors.Is(err, check.expError) {
					t.Fatalf("Expected %v, got %v", check.expError, err)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			} else if actual != check.expValue {
				t.Errorf("expected [%s], got [%v]", check.expValue, actual)
			}
		})
	}
}
func TestGetCredentials(t *testing.T) {
	checks := [...]struct {
		name             string
		args             []string
		expUser, expPass string
		expError         bool
	}{
		{"nothing, nil", nil, "", "", false},
		{"bad dsn", []string{"-dsn", ":localhost/0"}, "", "", true},
		{"dsn, no info", []string{"-dsn", "redis://localhost/0"}, "", "", false},
		{"dsn, only pass", []string{"-dsn", "redis://pass@localhost/0"}, "", "pass", false},
		{"dsn, only empty pass", []string{"-dsn", "redis://@localhost/0"}, "", "", false},
		{"dsn, user+pass", []string{"-dsn", "redis://foo:bar@localhost/0"}, "foo", "bar", false},
		{"dsn, user override", []string{"-dsn", "redis://foo:bar@localhost/0", "-user", "u"}, "u", "bar", false},
		{"dsn, pass override", []string{"-dsn", "redis://foo:bar@localhost/0", "-pass", "p"}, "foo", "p", false},
		// Expect error because, during tests, stdIn is not a terminal
		{"dsn, empty pass flag", []string{"-dsn", "redis://localhost/0", "-pass", ""}, "", "", true},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			fs := flag.NewFlagSet("test", flag.ContinueOnError)
			userFlag := fs.String("user", "", "")
			passFlag := fs.String("pass", "", "")
			dsnFlag := fs.String("dsn", "", "")
			err := fs.Parse(check.args)
			if err != nil && !check.expError {
				t.Fatalf("Unexpected error: %v", err)
			}
			user, pass, err := getCredentials(fs, io.Discard, *dsnFlag, *userFlag, *passFlag)
			// Redis interprets redis://foo@host as having foo for password, not for user,
			// unlike normal URL auth parsing.
			if err != nil && !check.expError {
				t.Fatalf("Unexpected error: %v", err)
			}
			if user != check.expUser {
				t.Errorf("got user [%s], expected [%s]", user, check.expUser)
			}
			if pass != check.expPass {
				t.Errorf("got pass [%s], expected [%s]", pass, check.expPass)
			}
		})
	}
}

// clientInfo returns the user a connection is authenticated as, and its database,
// as the server reports them.
func clientInfo(c redis.Conn) (user string, db int, err error) {
	info, err := redis.String(c.Do("CLIENT", "INFO"))
	if err != nil {
		return "", 0, err
	}
	for field := range strings.FieldsSeq(info) {
		switch k, v, _ := strings.Cut(field, "="); k {
		case "user":
			user = v
		case "db":
			if db, err = strconv.Atoi(v); err != nil {
				return "", 0, fmt.Errorf("invalid db in CLIENT INFO %q: %w", info, err)
			}
		}
	}
	return user, db, nil
}

// TestOpen checks the user and database of the connection for each way to
// pass credentials, as main does: getCredentials, then open.
func TestOpen(t *testing.T) {
	for _, s := range redistest.Servers(t) {
		t.Run(s.Image, func(t *testing.T) {
			t.Parallel()
			aclUser, aclPass := s.User(t, "~*", "+@all")
			const defaultUser = "default" // The requirepass user.
			checks := [...]struct {
				name     string
				userinfo string
				args     []string
				expUser  string // Empty when authentication must fail.
			}{
				{"dsn, only pass", redistest.Password + "@", nil, defaultUser},
				{"dsn, empty user and pass", ":" + redistest.Password + "@", nil, defaultUser},
				{"dsn, user and pass", aclUser + ":" + aclPass + "@", nil, aclUser},
				{"dsn, wrong pass", aclUser + ":wrong@", nil, ""},
				{"pass flag", "", []string{"-pass", redistest.Password}, defaultUser},
				{"user and pass flags", "", []string{"-user", aclUser, "-pass", aclPass}, aclUser},
				{"pass flag overriding the dsn", aclUser + ":wrong@", []string{"-pass", aclPass}, aclUser},
				{"user and pass flags overriding the dsn", redistest.Password + "@", []string{"-user", aclUser, "-pass", aclPass}, aclUser},
				{"no credentials", "", nil, ""},
			}
			for _, check := range checks {
				// Opening only reads: no database needs reserving.
				for _, db := range []int{0, 1} {
					t.Run(fmt.Sprintf("%s, db %d", check.name, db), func(t *testing.T) {
						t.Parallel()
						fs := flag.NewFlagSet("test", flag.ContinueOnError)
						userFlag := fs.String("user", "", "")
						passFlag := fs.String("pass", "", "")
						dsnFlag := fs.String("dsn", "", "")
						dsn := fmt.Sprintf("redis://%s%s/%d", check.userinfo, s.Addr(), db)
						if err := fs.Parse(append([]string{"-dsn", dsn}, check.args...)); err != nil {
							t.Fatalf("failed parsing flags: %v", err)
						}
						user, pass, err := getCredentials(fs, io.Discard, *dsnFlag, *userFlag, *passFlag)
						if err != nil {
							t.Fatalf("failed obtaining credentials: %v", err)
						}

						c, err := open(*dsnFlag, user, pass)
						if err == nil {
							defer c.Close()
							var actualUser string
							var actualDB int
							// Unauthenticated on DB 0, open succeeds, as it sends no command:
							// the first command fails instead.
							if actualUser, actualDB, err = clientInfo(c); err == nil {
								if actualUser != check.expUser || actualDB != db {
									t.Errorf("got user %q on db %d, expected %q on db %d", actualUser, actualDB, check.expUser, db)
								}
							}
						}
						if expErr := check.expUser == ""; (err != nil) != expErr {
							t.Errorf("got error %v, expected one: %t", err, expErr)
						}
					})
				}
			}
		})
	}
}

// TestOpenInvalidDSN checks that open rejects each kind of invalid DSN,
// and reports where it failed: its own parsing, redigo's, or the server.
//
// Parsing in open is only reached when both -user and -pass are given,
// since getCredentials parses the DSN otherwise.
func TestOpenInvalidDSN(t *testing.T) {
	for _, s := range redistest.Servers(t) {
		t.Run(s.Image, func(t *testing.T) {
			t.Parallel()
			checks := [...]struct {
				name, dsn    string
				expPrefix    string
				expSubstring string
			}{
				{"unparsable", ":localhost/0", "failed parsing Redis DSN", "missing protocol scheme"},
				{"wrong scheme", "http://" + s.Addr() + "/0", "failed dialing Redis URL", "invalid redis URL scheme"},
				{"non-numeric database", "redis://" + s.Addr() + "/x", "failed dialing Redis URL", "invalid database"},
				// The servers have databases 0 to 63.
				{"database out of range", "redis://" + s.Addr() + "/64", "failed dialing Redis URL", "DB index is out of range"},
				// Port 1 is privileged, and its TCPMUX service is obsolete: nothing listens on it.
				{"nothing listening", "redis://127.0.0.1:1/0", "failed dialing Redis URL", "connection refused"},
			}
			for _, check := range checks {
				t.Run(check.name, func(t *testing.T) {
					c, err := open(check.dsn, "", redistest.Password)
					if err == nil {
						c.Close()
						t.Fatal("expected an error")
					}
					if msg := err.Error(); !strings.HasPrefix(msg, check.expPrefix) || !strings.Contains(msg, check.expSubstring) {
						t.Errorf("got error %q, expected %q, containing %q", msg, check.expPrefix, check.expSubstring)
					}
				})
			}
		})
	}
}
