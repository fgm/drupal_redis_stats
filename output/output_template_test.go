package output_test

// This file is derived from the Go 1.16 html/template/exec_test.go,
// under its 3-clause BSD + patent grant license

import "errors"

const alwaysErrorText = "always be failing"

var ErrAlways = errors.New(alwaysErrorText)

type ErrorWriter int

func (e ErrorWriter) Write(p []byte) (int, error) {
	return 0, ErrAlways
}
