//go:build !windows && !darwin

package scanner

import (
	"context"
	"fmt"
	"runtime"
)

type unsupportedScanner struct{}

// New returns a scanner that errors out: only Windows and macOS are supported
// in v1 (spec module 1, "Out of scope").
func New() Scanner { return &unsupportedScanner{} }

func (unsupportedScanner) Scan(context.Context) ([]Software, error) {
	return nil, fmt.Errorf("software scanning is not supported on %s", runtime.GOOS)
}
