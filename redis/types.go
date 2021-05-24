package redis

import (
	"strconv"

	"github.com/spf13/pflag"
)

type (
	// DSNValue is a wire-compatible string for DSN flags
	DSNValue string
	// JSONValue is a wire-compatible boolean for jsonOutput
	JSONValue bool
	// PassValue is a wire-compatible string for passwords
	PassValue string
	// QuietValue is a wire-compatible boolean for quiet
	QuietValue bool
	// UserValue is a wire-compatible string for user names
	UserValue string
)

// String is part of the pflag.Value implementation
func (d *DSNValue) String() string { return string(*d) }

// Set is part of the pflag.Value implementation
func (d *DSNValue) Set(s string) error { *d = DSNValue(s); return nil }

// Type is part of the pflag.Value implementation
func (d *DSNValue) Type() string { return "string" }

// String is part of the pflag.Value implementation
func (d *PassValue) String() string { return string(*d) }

// Set is part of the pflag.Value implementation
func (d *PassValue) Set(s string) error { *d = PassValue(s); return nil }

// Type is part of the pflag.Value implementation
func (d *PassValue) Type() string { return "string" }

// String is part of the pflag.Value implementation
func (d *UserValue) String() string { return string(*d) }

// Set is part of the pflag.Value implementation
func (d *UserValue) Set(s string) error { *d = UserValue(s); return nil }

// Type is part of the pflag.Value implementation
func (d *UserValue) Type() string { return "string" }

// String is part of the pflag.Value implementation
func (d *JSONValue) String() string { return strconv.FormatBool(bool(*d)) }

// Set is part of the pflag.Value implementation
func (d *JSONValue) Set(s string) error {
	b, err := strconv.ParseBool(s)
	*d = JSONValue(b)
	return err
}

// Type is part of the pflag.Value implementation
func (d *JSONValue) Type() string { return "bool" }

// String is part of the pflag.Value implementation
func (d *QuietValue) String() string { return strconv.FormatBool(bool(*d)) }

// Set is part of the pflag.Value implementation
func (d *QuietValue) Set(s string) error {
	b, err := strconv.ParseBool(s)
	*d = QuietValue(b)
	return err
}

// Type is part of the pflag.Value implementation
func (d *QuietValue) Type() string { return "bool" }

// IsFlagPassed reports whether or not a named flag has been passed.
func IsFlagPassed(fs *pflag.FlagSet, name string) bool {
	found := false
	fs.Visit(func(f *pflag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}
