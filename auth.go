package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"unicode/utf8"

	"github.com/gomodule/redigo/redis"
	"golang.org/x/term"
)

type PasswordReader interface {
	ReadPassword(int) ([]byte, error)
}

type passwordReaderFunc func(int) ([]byte, error)

func (prf passwordReaderFunc) ReadPassword(fd int) ([]byte, error) {
	return prf(fd)
}

var ErrInvalidUTF8 = errors.New("input was not a valid UTF-8 string")

func getPasswordFromCLI(w io.Writer, pr PasswordReader, fd int) (string, error) {
	fmt.Fprint(w, "Password: ")
	pBytes, err := pr.ReadPassword(fd)
	if err != nil {
		return "", fmt.Errorf("failed acquiring password from terminal without echo: %w", err)
	}
	if !utf8.Valid(pBytes) {
		return "", fmt.Errorf("invalid password: %w", ErrInvalidUTF8)
	}
	return string(pBytes), nil
}

// getCredentials combines the user and pass values with those possibly
// set in the DSN. If the pass flag was set but empty, it read it from the
// terminal in echo-less mode.
//
// Explicit user/pass flags override those found in the DSN.
func getCredentials(fs *flag.FlagSet, w io.Writer, flagDSN, flagUser, flagPass string) (user, pass string, err error) {
	user, pass = flagUser, flagPass

	hasDSNFlag := isFlagPassed(fs, "dsn")
	hasUserFlag := isFlagPassed(fs, "user")
	hasPassFlag := isFlagPassed(fs, "pass")

	if hasDSNFlag && (!hasUserFlag || !hasPassFlag) {
		var u *url.URL

		u, err = url.Parse(flagDSN)
		if err != nil {
			return "", "", fmt.Errorf("failed parsing Redis DSN: %v", err)
		}
		if !hasUserFlag {
			user = u.User.Username()
		}
		if !hasPassFlag {
			dsnPass, DSNContainsPass := u.User.Password()
			if DSNContainsPass {
				pass = dsnPass
			} else {
				// Redis URL parsing accepts single auth element as password, not user
				pass = user
				user = ""
			}
		}
	}

	if hasPassFlag && flagPass == "" {
		pass, err = getPasswordFromCLI(w, passwordReaderFunc(term.ReadPassword), int(os.Stdin.Fd()))
		if err != nil {
			return "", "", err
		}
	}
	return
}

func authenticate(c redis.Conn, includeUser bool, user, pass string) error {
	var err error
	if includeUser {
		_, err = c.Do("AUTH", user, pass)
	} else {
		_, err = c.Do("AUTH", pass)
	}
	if err != nil {
		return fmt.Errorf("failed AUTH: %w", err)
	}

	return nil
}
