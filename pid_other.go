//go:build !unix

package main

import (
	"context"
	"fmt"
	"runtime"
)

func (t *pidTarget) Check(context.Context) error {
	return fmt.Errorf("pid targets are not supported on %s", runtime.GOOS)
}
