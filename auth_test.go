package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"syscall"
	"testing"

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

			actual, err := getPasswordFromCLI(io.Discard, check.reader, int(f.Fd()))
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

type MockConn struct {
	RequirePass bool
	User, Pass  string
}

func (mc *MockConn) Close() error { return nil }

// Err returns a non-nil value when the connection is not usable.
func (mc *MockConn) Err() error { return nil }

// Do sends a command to the server and returns the received reply.
func (mc *MockConn) Do(commandName string, args ...interface{}) (reply interface{}, err error) {
	if commandName != "AUTH" {
		return nil, errors.New("bad command")
	}
	switch {
	case mc.RequirePass:
		if len(args) != 1 {
			return nil, fmt.Errorf("requirepass only accepts 1 argument for the password, got %d", len(args))
		}
		if args[0] == mc.Pass {
			return "OK", nil
		}
		return "Bad password", errors.New("incorrect password")
	case len(args) == 0:
		return nil, errors.New("no argument, needs at least one")
	case len(args) != 2:
		return nil, fmt.Errorf("argument count: %d, ACL mode needs 2", len(args))
	default:
		if args[0] == mc.User && args[1] == mc.Pass {
			return "OK", nil
		}
		return nil, errors.New("incorrect user or password")
	}
}

// Send writes the command to the client's output buffer.
func (mc *MockConn) Send(commandName string, args ...interface{}) error { return nil }

// Flush flushes the output buffer to the Redis server.
func (mc *MockConn) Flush() error { return nil }

// Receive receives a single reply from the Redis server
func (mc *MockConn) Receive() (reply interface{}, err error) { return nil, nil }

func TestAuthenticate(t *testing.T) {
	checks := [...]struct {
		name             string
		includeUser      bool
		user, pass       string
		requirePass      bool // redis.conf requirepass = true
		aclUser, aclPass string
		expErr           bool
	}{
		{"requirepass, only pass", false, "", "pass", true, "", "pass", false},
		{"requirepass, no pass", false, "", "", true, "", "pass", true},
		{"acl, only good user", true, "user", "", false, "user", "pass", true},
		{"acl, only good pass", true, "", "pass", false, "user", "pass", true},
		{"acl, both good", true, "user", "pass", false, "user", "pass", false},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			conn := &MockConn{
				RequirePass: check.requirePass,
				User:        check.aclUser,
				Pass:        check.aclPass,
			}
			err := authenticate(conn, check.includeUser, check.user, check.pass)
			if err != nil && !check.expErr {
				t.Fatalf("unexpected error: %v", err)
			} else if err == nil && check.expErr {
				t.Fatal("unexpected success")
			}
		})
	}
}
